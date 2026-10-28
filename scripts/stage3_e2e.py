#!/usr/bin/env python3
"""Stage-three API smoke/E2E checks. Reads ADMIN_* from the project .env file."""

from __future__ import annotations

import concurrent.futures
import json
import os
import time
import urllib.error
import urllib.request
from pathlib import Path

BASE_URL = os.environ.get("CABINET_API_BASE", "http://127.0.0.1:8080")
PROJECT_DIR = Path(os.environ.get("CABINET_PROJECT_DIR", "/home/agic/workspace/omnirack-replica"))


def read_env(path: Path) -> dict[str, str]:
    values: dict[str, str] = {}
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        values[key.strip()] = value.strip().strip('"').strip("'")
    return values


def raw_request(method: str, path: str, payload=None, token: str | None = None):
    body = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    headers = {"Accept": "application/json"}
    if body is not None:
        headers["Content-Type"] = "application/json"
    if token:
        headers["Authorization"] = f"Bearer {token}"
    request = urllib.request.Request(BASE_URL + path, data=body, headers=headers, method=method)
    try:
        with urllib.request.urlopen(request, timeout=20) as response:
            content = json.loads(response.read().decode("utf-8"))
            return response.status, content
    except urllib.error.HTTPError as error:
        content = json.loads(error.read().decode("utf-8"))
        return error.code, content


def expect(method: str, path: str, payload=None, token: str | None = None, statuses=(200,)):
    status, content = raw_request(method, path, payload, token)
    if status not in statuses:
        raise AssertionError(f"{method} {path}: HTTP {status}, body={content}")
    return content.get("data"), content


def expect_error(method: str, path: str, expected_code: str, payload=None, token: str | None = None, status=409):
    actual_status, content = raw_request(method, path, payload, token)
    if actual_status != status or content.get("code") != expected_code:
        raise AssertionError(
            f"{method} {path}: expected HTTP {status}/{expected_code}, got HTTP {actual_status}/{content.get('code')}, body={content}"
        )
    return content


def position_payload(device: dict, rack_id: str, face: str, start_u: int, height_u: int | None = None):
    return {
        "version": device["version"],
        "rackId": rack_id,
        "face": face,
        "startU": start_u,
        "heightU": height_u or device["heightU"],
        "orientation": "NORMAL",
        "reason": "stage3-e2e",
    }


def main() -> None:
    env = read_env(PROJECT_DIR / ".env")
    username = env.get("ADMIN_USERNAME", "admin")
    password = env["ADMIN_PASSWORD"]
    login, _ = expect("POST", "/api/v1/auth/login", {"username": username, "password": password}, statuses=(200,))
    token = login["token"]
    suffix = str(int(time.time()))[-8:]
    created_devices: list[str] = []
    dc_id = room_id = rack_id = None

    def api(method: str, path: str, payload=None, statuses=(200,)):
        return expect(method, path, payload, token, statuses)

    def create_device(code_suffix: str, type_id: str, height_u: int = 1):
        data, _ = api(
            "POST",
            "/api/v1/devices",
            {
                "typeId": type_id,
                "code": f"E2E-{suffix}-{code_suffix}",
                "name": f"阶段三验证设备 {code_suffix}",
                "lifecycleStatus": "WAITING_RACK",
                "heightU": height_u,
            },
            (201,),
        )
        created_devices.append(data["id"])
        return data

    def get_device(device_id: str):
        data, _ = api("GET", f"/api/v1/devices/{device_id}")
        return data

    def set_policy(assign: bool, move: bool):
        policy, _ = api("GET", "/api/v1/settings/operation-policy")
        updated, _ = api(
            "PUT",
            "/api/v1/settings/operation-policy",
            {
                "version": policy["version"],
                "assignApprovalEnabled": assign,
                "moveApprovalEnabled": move,
                "defaultApproverRole": "system_admin",
            },
        )
        return updated

    try:
        types, _ = api("GET", "/api/v1/device-types")
        if len(types["items"]) < 6:
            raise AssertionError("默认设备类型不足 6 个")
        type_id = types["items"][0]["id"]
        print("PASS 01 默认设备类型已生成")

        policy, _ = api("GET", "/api/v1/settings/operation-policy")
        if policy["assignApprovalEnabled"] or policy["moveApprovalEnabled"]:
            set_policy(False, False)
        print("PASS 02 审批默认/当前关闭并已归零")

        dc, _ = api("POST", "/api/v1/data-centers", {"code": f"E2EDC{suffix}", "name": "阶段三验证数据中心", "status": "OPERATING"}, (201,))
        dc_id = dc["id"]
        room, _ = api("POST", f"/api/v1/data-centers/{dc_id}/rooms", {"code": f"R{suffix}", "name": "阶段三验证机房", "status": "OPERATING", "floorPlanFormat": "SVG"}, (201,))
        room_id = room["id"]
        rack, _ = api("POST", f"/api/v1/rooms/{room_id}/racks", {"code": f"RK{suffix}", "name": "阶段三验证机柜", "status": "AVAILABLE", "uHeight": 42, "widthMm": 600, "depthMm": 1200, "heightMm": 2000, "rotation": 0}, (201,))
        rack_id = rack["id"]
        print("PASS 03 创建数据中心/机房/42U 机柜")

        device1 = create_device("CORE-1", type_id, 2)
        device2 = create_device("CORE-2", type_id, 2)
        device3 = create_device("CORE-3", type_id, 1)
        print("PASS 04 创建设备台账")

        result, _ = api("POST", f"/api/v1/devices/{device1['id']}/assign", position_payload(device1, rack_id, "FRONT", 10))
        if not result["executed"]:
            raise AssertionError("审批关闭时上架未立即执行")
        device1 = get_device(device1["id"])
        print("PASS 05 正面上架成功")

        expect_error("POST", f"/api/v1/devices/{device2['id']}/assign", "RACK_U_CONFLICT", position_payload(device2, rack_id, "FRONT", 11), token)
        print("PASS 06 同面重叠返回 RACK_U_CONFLICT")

        result, _ = api("POST", f"/api/v1/devices/{device2['id']}/assign", position_payload(device2, rack_id, "REAR", 10))
        if not result["executed"]:
            raise AssertionError("背面上架未执行")
        device2 = get_device(device2["id"])
        print("PASS 07 相同 U 位背面可独立上架")

        expect_error("POST", f"/api/v1/devices/{device3['id']}/assign", "RACK_U_CONFLICT", position_payload(device3, rack_id, "BOTH", 10), token)
        result, _ = api("POST", f"/api/v1/devices/{device3['id']}/assign", position_payload(device3, rack_id, "BOTH", 20))
        if not result["executed"]:
            raise AssertionError("BOTH 上架未执行")
        device3 = get_device(device3["id"])
        blocker = create_device("BOTH-BLOCK", type_id, 1)
        expect_error("POST", f"/api/v1/devices/{blocker['id']}/assign", "RACK_U_CONFLICT", position_payload(blocker, rack_id, "FRONT", 20), token)
        print("PASS 08 BOTH 与正面/背面均冲突")

        result, _ = api("POST", f"/api/v1/devices/{device1['id']}/move", position_payload(device1, rack_id, "FRONT", 30))
        if not result["executed"]:
            raise AssertionError("迁移未立即执行")
        device1 = get_device(device1["id"])
        print("PASS 09 设备迁移成功")

        history, _ = api("GET", f"/api/v1/devices/{device1['id']}/history")
        if [item["operation"] for item in history["items"]][:2] != ["MOVE", "ASSIGN"]:
            raise AssertionError(f"履历顺序或内容异常: {history['items']}")
        api("POST", f"/api/v1/devices/{device1['id']}/decommission", {"version": device1["version"], "reason": "E2E 下架"})
        device1 = get_device(device1["id"])
        history, _ = api("GET", f"/api/v1/devices/{device1['id']}/history")
        if len(history["items"]) < 3 or history["items"][0]["operation"] != "DECOMMISSION":
            raise AssertionError("下架履历未生成")
        print("PASS 10 下架成功且设备履历完整")

        original = create_device("LOCK", type_id, 1)
        first_payload = dict(original)
        first_payload["name"] = "乐观锁更新成功"
        updated, _ = api("PUT", f"/api/v1/devices/{original['id']}", first_payload)
        stale_payload = dict(original)
        stale_payload["name"] = "过期版本不应成功"
        expect_error("PUT", f"/api/v1/devices/{original['id']}", "RESOURCE_VERSION_CONFLICT", stale_payload, token)
        print("PASS 11 乐观锁冲突返回 409")

        set_policy(True, True)
        approval_device = create_device("APPROVAL", type_id, 1)
        pending_result, _ = api("POST", f"/api/v1/devices/{approval_device['id']}/assign", position_payload(approval_device, rack_id, "FRONT", 25))
        if pending_result["executed"] or not pending_result.get("approval"):
            raise AssertionError("开启上架审批后仍立即执行")
        if get_device(approval_device["id"]).get("currentPosition"):
            raise AssertionError("待审批申请提前修改了正式位置")
        approval = pending_result["approval"]
        api("POST", f"/api/v1/approvals/{approval['id']}/approve", {"version": approval["version"], "comment": "E2E 通过"})
        approval_device = get_device(approval_device["id"])
        if approval_device.get("currentPosition", {}).get("startU") != 25:
            raise AssertionError("审批通过后未执行上架")
        move_result, _ = api("POST", f"/api/v1/devices/{approval_device['id']}/move", position_payload(approval_device, rack_id, "FRONT", 26))
        move_approval = move_result.get("approval")
        if move_result["executed"] or not move_approval:
            raise AssertionError("开启迁移审批后仍立即执行")
        api("POST", f"/api/v1/approvals/{move_approval['id']}/approve", {"version": move_approval["version"], "comment": "E2E 迁移通过"})
        approval_device = get_device(approval_device["id"])
        if approval_device.get("currentPosition", {}).get("startU") != 26:
            raise AssertionError("迁移审批通过后未执行")
        api("POST", f"/api/v1/devices/{approval_device['id']}/decommission", {"version": approval_device["version"], "reason": "E2E cleanup"})
        print("PASS 12 单级上架/迁移审批按配置生效")

        set_policy(True, False)
        conflict_pending_device = create_device("APPROVAL-CONFLICT", type_id, 1)
        conflict_pending_result, _ = api("POST", f"/api/v1/devices/{conflict_pending_device['id']}/assign", position_payload(conflict_pending_device, rack_id, "FRONT", 35))
        conflict_approval = conflict_pending_result["approval"]
        device2 = get_device(device2["id"])
        api("POST", f"/api/v1/devices/{device2['id']}/move", position_payload(device2, rack_id, "FRONT", 35))
        expect_error("POST", f"/api/v1/approvals/{conflict_approval['id']}/approve", "RACK_U_CONFLICT", {"version": conflict_approval["version"], "comment": "应冲突"}, token)
        api("POST", f"/api/v1/approvals/{conflict_approval['id']}/reject", {"version": conflict_approval["version"], "comment": "E2E 清理"})
        print("PASS 13 审批通过时重新校验目标 U 位")
        set_policy(False, False)

        concurrent_a = create_device("CONCURRENT-A", type_id, 1)
        concurrent_b = create_device("CONCURRENT-B", type_id, 1)
        calls = [
            (concurrent_a, position_payload(concurrent_a, rack_id, "FRONT", 40)),
            (concurrent_b, position_payload(concurrent_b, rack_id, "FRONT", 40)),
        ]
        with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
            futures = [executor.submit(raw_request, "POST", f"/api/v1/devices/{device['id']}/assign", payload, token) for device, payload in calls]
            outcomes = [future.result() for future in futures]
        statuses = sorted(status for status, _ in outcomes)
        codes = sorted(content.get("code") for _, content in outcomes)
        if statuses != [200, 409] or codes != ["RACK_U_CONFLICT", "SUCCESS"]:
            raise AssertionError(f"并发占位结果异常: {outcomes}")
        for device in (concurrent_a, concurrent_b):
            current = get_device(device["id"])
            if current.get("currentPosition"):
                api("POST", f"/api/v1/devices/{device['id']}/decommission", {"version": current["version"], "reason": "E2E cleanup"})
        print("PASS 14 并发相同 U 位最终仅一个成功")

        fillers = []
        for index in range(1, 19):
            item = create_device(f"FILL-{index:02d}", type_id, 1)
            result, _ = api("POST", f"/api/v1/devices/{item['id']}/assign", position_payload(item, rack_id, "FRONT", index))
            if not result["executed"]:
                raise AssertionError("填充设备上架未执行")
            fillers.append(item["id"])
        overflow = create_device("RACK-LIMIT", type_id, 1)
        expect_error("POST", f"/api/v1/devices/{overflow['id']}/assign", "RESOURCE_LIMIT_EXCEEDED", position_payload(overflow, rack_id, "FRONT", 19), token)
        print("PASS 15 每柜第 21 台设备被拒绝")

        tree, _ = api("GET", "/api/v1/resource-tree")
        rack_version = next(r["version"] for dc_item in tree["items"] for room_item in dc_item.get("rooms", []) for r in room_item.get("racks", []) if r["id"] == rack_id)
        expect_error("DELETE", f"/api/v1/racks/{rack_id}?version={rack_version}", "RESOURCE_HAS_CHILDREN", token=token)
        print("PASS 16 有设备的机柜禁止删除")

        layout, _ = api("GET", f"/api/v1/racks/{rack_id}/u-layout")
        if not layout.get("positions") or "frontFree" not in layout or "rearFree" not in layout:
            raise AssertionError("U 位视图数据不完整")
        print("PASS 17 正面/背面 U 位布局接口完整")

        print("阶段三 API E2E：17/17 通过")
    finally:
        if token:
            try:
                set_policy(False, False)
            except Exception:
                pass
            for device_id in created_devices:
                try:
                    current = get_device(device_id)
                    if current.get("currentPosition"):
                        api("POST", f"/api/v1/devices/{device_id}/decommission", {"version": current["version"], "reason": "stage3-e2e cleanup"})
                    current = get_device(device_id)
                    api("DELETE", f"/api/v1/devices/{device_id}?version={current['version']}")
                except Exception:
                    pass
            if rack_id:
                try:
                    tree, _ = api("GET", "/api/v1/resource-tree")
                    dc_item = next(item for item in tree["items"] if item["id"] == dc_id)
                    room_item = next(item for item in dc_item.get("rooms", []) if item["id"] == room_id)
                    rack_item = next(item for item in room_item.get("racks", []) if item["id"] == rack_id)
                    api("DELETE", f"/api/v1/racks/{rack_id}?version={rack_item['version']}")
                    tree, _ = api("GET", "/api/v1/resource-tree")
                    dc_item = next(item for item in tree["items"] if item["id"] == dc_id)
                    room_item = next(item for item in dc_item.get("rooms", []) if item["id"] == room_id)
                    api("DELETE", f"/api/v1/rooms/{room_id}?version={room_item['version']}")
                    tree, _ = api("GET", "/api/v1/resource-tree")
                    dc_item = next(item for item in tree["items"] if item["id"] == dc_id)
                    api("DELETE", f"/api/v1/data-centers/{dc_id}?version={dc_item['version']}")
                except Exception:
                    pass


if __name__ == "__main__":
    main()
