"""Additional real HTTP business loops; called only by the owned disposable runner.

No remote URLs, database credentials or production instance switches are accepted.
Covers PDU, quantity-stock transfer, inventory difference/close and approval execution.
This is a defined regression fixture, not exhaustive business equivalence.
"""

import secrets
import uuid


def stock_transfer_workflow(api):
    source = api("POST", "/asset-ops/sites", {"region": "闭环", "name": "调拨源库", "kind": "STORE"}, 201)
    target = api("POST", "/asset-ops/sites", {"region": "闭环", "name": "调拨目标库", "kind": "STORE"}, 201)
    stock = api("POST", "/asset-ops/stock-receipts", {"submissionId": str(uuid.uuid4()), "siteId": source["id"], "category": "MEMORY", "name": "闭环数量备件", "quantity": 10, "notes": "隔离调拨夹具"})

    def stocks():
        return {item["id"]: item for item in api("GET", "/asset-ops/stocks")["items"] if item["specId"] == stock["specId"]}

    command = {"version": stock["version"], "submissionId": str(uuid.uuid4()), "operation": "TRANSFER", "quantity": 4, "destinationSiteId": target["id"], "reason": "隔离调拨验证"}
    movement = api("POST", f"/asset-ops/stocks/{stock['id']}/move", command)
    assert movement["beforeQuantity"] == 10 and movement["afterQuantity"] == 6
    destination_id = movement["destinationStockId"]
    state = stocks()
    assert len(state) == 2 and sum(item["quantity"] for item in state.values()) == 10
    assert state[stock["id"]]["quantity"] == 6 and state[stock["id"]]["siteId"] == source["id"]
    transit = state[destination_id]
    assert transit["quantity"] == 4 and transit["state"] == "TRANSIT" and transit["destinationSiteId"] == target["id"]
    replay = api("POST", f"/asset-ops/stocks/{stock['id']}/move", command)
    assert replay["id"] == movement["id"] and stocks() == state
    api("POST", f"/asset-ops/stocks/{stock['id']}/move", {**command, "quantity": 3}, 400)
    api("POST", f"/asset-ops/stocks/{stock['id']}/move", {**command, "submissionId": str(uuid.uuid4())}, 409)
    assert stocks() == state
    receive = {"version": transit["version"], "submissionId": str(uuid.uuid4()), "operation": "RECEIVE", "quantity": 4, "reason": "目标库确认收货"}
    api("POST", f"/asset-ops/stocks/{destination_id}/move", {**receive, "quantity": 3}, 400)
    received = api("POST", f"/asset-ops/stocks/{destination_id}/move", receive)
    final = stocks()
    assert sum(item["quantity"] for item in final.values()) == 10
    assert final[destination_id]["state"] == "AVAILABLE" and final[destination_id]["siteId"] == target["id"]
    assert not final[destination_id].get("destinationSiteId")
    assert api("POST", f"/asset-ops/stocks/{destination_id}/move", receive)["id"] == received["id"]
    api("POST", f"/asset-ops/stocks/{destination_id}/move", {**receive, "submissionId": str(uuid.uuid4()), "version": final[destination_id]["version"]}, 400)
    assert stocks() == final
    movements = api("GET", "/asset-ops/movements")["items"]
    transfers = [item for item in movements if item["stockId"] == stock["id"] and item["operation"] == "TRANSFER"]
    receipts = [item for item in movements if item["stockId"] == destination_id and item["operation"] == "RECEIVE"]
    assert len(transfers) == len(receipts) == 1
    assert transfers[0]["destinationStockId"] == destination_id and receipts[0]["afterState"] == "AVAILABLE"


def inventory_close_workflow(api):
    admin = api("GET", "/auth/me")
    # A separate authenticated actor is necessary: an assignee cannot review their own difference.
    password = secrets.token_urlsafe(24)
    api("POST", "/admin/users", {"username": "inventory-reviewer", "password": password, "displayName": "隔离复核人", "enabled": True, "roleCodes": ["asset_manager"]}, 201)
    reviewer = api("POST", "/auth/login", {"username": "inventory-reviewer", "password": password})["token"]
    site = api("POST", "/asset-ops/sites", {"region": "闭环", "name": "盘点独立库", "kind": "STORE"}, 201)
    stock = api("POST", "/asset-ops/stock-receipts", {"submissionId": str(uuid.uuid4()), "siteId": site["id"], "category": "MEMORY", "name": "盘点差异备件", "quantity": 5})
    task = api("POST", "/asset-ops/checks", {"name": "差异关闭闭环", "siteId": site["id"], "assigneeId": admin["id"]}, 201)
    path = f"/asset-ops/checks/{task['id']}"
    lines = api("GET", path + "/lines")["items"]
    assert len(lines) == 1 and lines[0]["targetId"] == stock["id"] and lines[0]["expected"] == 5
    line = lines[0]
    api("POST", path + "/close", expected=400)
    scan = {"lineId": line["id"], "actual": 3, "difference": True, "note": "现场短缺两件"}
    api("POST", path + "/scan", scan, 400, reviewer)
    scanned = api("POST", path + "/scan", scan)
    assert not scanned["duplicate"] and scanned["line"]["result"] == "DIFFERENCE" and scanned["line"]["actual"] == 3
    repeated = api("POST", path + "/scan", {**scan, "actual": 1})
    assert repeated["duplicate"] and repeated["line"]["actual"] == 3
    api("POST", path + "/close", expected=400)
    resolution = {"resolution": "独立复核实物与账目，批准调整", "adjustQuantity": True}
    resolve_path = path + f"/lines/{line['id']}/resolve"
    api("POST", resolve_path, resolution, 400)
    resolved = api("POST", resolve_path, resolution, auth=reviewer)
    assert resolved.get("resolvedBy") and resolved["resolution"] == resolution["resolution"]
    current = next(item for item in api("GET", "/asset-ops/stocks")["items"] if item["id"] == stock["id"])
    assert current["quantity"] == 3 and current["version"] > stock["version"]
    api("POST", resolve_path, resolution, 400, reviewer)
    resolved_snapshot = api("GET", path + "/lines")["items"]
    assert resolved_snapshot[0]["resolvedBy"] == resolved["resolvedBy"] and resolved_snapshot[0]["actual"] == 3
    assert api("POST", path + "/close")["closed"]
    closed = next(item for item in api("GET", "/asset-ops/checks")["items"] if item["id"] == task["id"])
    assert closed["state"] == "CLOSED" and closed.get("closedAt")
    api("POST", path + "/scan", scan, 400)
    api("POST", resolve_path, resolution, 400, reviewer)
    api("POST", path + "/close", expected=400)
    after = api("GET", path + "/lines")["items"]
    assert after == resolved_snapshot
    adjustments = [item for item in api("GET", "/asset-ops/movements")["items"] if item["stockId"] == stock["id"] and item["operation"] == "ADJUST"]
    assert len(adjustments) == 1 and adjustments[0]["beforeQuantity"] == 5 and adjustments[0]["afterQuantity"] == 3


def approval_execution_workflow(api, room):
    password = secrets.token_urlsafe(24)
    api("POST", "/admin/users", {"username": "workflow-approver", "password": password, "displayName": "隔离审批人", "enabled": True, "roleCodes": ["system_admin"]}, 201)
    approver = api("POST", "/auth/login", {"username": "workflow-approver", "password": password})["token"]
    policy = api("GET", "/admin/approval-policy")
    api("PUT", "/admin/approval-policy", {"version": policy["version"], "assignApprovalEnabled": True, "moveApprovalEnabled": True, "decommissionApprovalEnabled": True, "defaultApproverRole": "system_admin"})
    rack = api("POST", f"/rooms/{room['id']}/racks", {"name": "审批闭环机柜", "autoGenerateCode": True, "uHeight": 42}, 201)
    server_type = next(item for item in api("GET", "/device-types")["items"] if item["category"] == "SERVER")
    device = api("POST", "/devices", {"name": "审批执行设备", "typeId": server_type["id"], "heightU": 1, "autoGenerateCode": True}, 201)
    def current():
        return api("GET", f"/devices/{device['id']}")
    def decision(record, action, expected=200, version=None):
        return api("POST", f"/admin/approvals/{record['id']}/{action}", {"version": record["version"] if version is None else version, "comment": "隔离审批闭环"}, expected, approver)
    command = {"version": device["version"], "rackId": rack["id"], "startU": 1, "heightU": 1, "orientation": "NORMAL", "reason": "隔离上架审批"}
    pending = api("POST", f"/devices/{device['id']}/assign", command)
    assert not pending["executed"] and pending["approval"]["status"] == "PENDING"
    assert not current().get("currentPosition") and current()["version"] == device["version"]
    record = pending["approval"]
    decision(record, "approve", 409, record["version"] + 99)
    assert not current().get("currentPosition")
    approved = decision(record, "approve")
    positioned = current()
    assert approved["status"] == "APPROVED" and approved.get("decidedBy")
    assert positioned["currentPosition"]["rackId"] == rack["id"] and positioned["currentPosition"]["startU"] == 1
    assert positioned["version"] > device["version"]
    decision(record, "approve", 409)
    decision(approved, "approve", 409)
    decision(approved, "reject", 409)
    assert current() == positioned
    move = api("POST", f"/devices/{device['id']}/move", {**command, "version": positioned["version"], "startU": 3, "reason": "拒绝移动不执行"})
    assert not move["executed"] and move["approval"]["status"] == "PENDING"
    assert current() == positioned
    rejected = decision(move["approval"], "reject")
    assert rejected["status"] == "REJECTED" and current() == positioned
    decision(rejected, "approve", 409)
    decision(rejected, "reject", 409)
    assert current() == positioned
    # Restore only this disposable database policy so later browser fixtures remain independent.
    updated = api("GET", "/admin/approval-policy")
    api("PUT", "/admin/approval-policy", {"version": updated["version"], "assignApprovalEnabled": policy["assignApprovalEnabled"], "moveApprovalEnabled": policy["moveApprovalEnabled"], "decommissionApprovalEnabled": policy["decommissionApprovalEnabled"], "defaultApproverRole": policy["defaultApproverRole"]})


def run_business_workflows(api, room, rack):
    types = api('GET', '/device-types')['items']
    server_type = next(item for item in types if item['category'] == 'SERVER')
    target_rack = api('POST', f"/rooms/{room['id']}/racks", {'name': '闭环验证-目标机柜', 'autoGenerateCode': True, 'uHeight': 42}, 201)
    device = api('POST', '/devices', {'name': '闭环验证-PDU设备', 'typeId': server_type['id'], 'heightU': 1, 'autoGenerateCode': True}, 201)
    api('POST', f"/devices/{device['id']}/assign", {'version': device['version'], 'rackId': rack['id'], 'startU': 1, 'heightU': 1, 'orientation': 'NORMAL', 'reason': '隔离PDU闭环'})
    device = api('GET', f"/devices/{device['id']}")
    assert device['currentPosition']['rackId'] == rack['id']
    pdu = api('POST', f"/racks/{rack['id']}/pdus", {'name': '闭环验证-PDU', 'autoGenerateCode': True, 'status': 'ACTIVE'}, 201)
    socket = api('POST', f"/pdus/{pdu['id']}/sockets", {'socketNo': 1, 'standard': 'CN', 'amperageA': 10, 'status': 'AVAILABLE'}, 201)
    command = {'version': socket['version'], 'deviceId': device['id'], 'powerW': 100, 'circuit': 'A', 'redundancyRole': 'PRIMARY'}
    connection = api('POST', f"/pdu-sockets/{socket['id']}/connection", command, 201)
    sockets = api('GET', f"/pdus/{pdu['id']}/sockets")['items']
    occupied = next(item for item in sockets if item['id'] == socket['id'])
    assert occupied['status'] == 'CONNECTED' and occupied['version'] > socket['version']
    api('POST', f"/pdu-sockets/{socket['id']}/connection", command, 409)
    api('POST', f"/devices/{device['id']}/move", {'version': device['version'], 'rackId': target_rack['id'], 'startU': 1, 'heightU': 1, 'orientation': 'NORMAL', 'reason': '带电跨柜必须拒绝'}, 400)
    assert api('GET', f"/devices/{device['id']}")['currentPosition']['rackId'] == rack['id']
    api('DELETE', f"/pdu-connections/{connection['id']}", {'version': connection['version'] + 99}, 409)
    assert len(api('GET', f"/racks/{rack['id']}/pdu-connections")['items']) == 1
    api('DELETE', f"/pdu-connections/{connection['id']}", {'version': connection['version']})
    released = next(item for item in api('GET', f"/pdus/{pdu['id']}/sockets")['items'] if item['id'] == socket['id'])
    assert released['status'] == 'AVAILABLE' and released['version'] > occupied['version']
    assert api('GET', f"/racks/{rack['id']}/pdu-connections")['items'] == []
    current = api('GET', f"/devices/{device['id']}")
    api('POST', f"/devices/{device['id']}/move", {'version': current['version'], 'rackId': target_rack['id'], 'startU': 1, 'heightU': 1, 'orientation': 'NORMAL', 'reason': '断电后跨柜验证'})
    assert api('GET', f"/devices/{device['id']}")['currentPosition']['rackId'] == target_rack['id']
    api('POST', f"/pdu-sockets/{socket['id']}/connection", {**command, 'version': released['version']}, 409)
    assert api('GET', f"/racks/{rack['id']}/pdu-connections")['items'] == []
    stock_transfer_workflow(api)
    inventory_close_workflow(api)
    approval_execution_workflow(api, room)
    return {'pdu': 'passed', 'stock_transfer': 'passed', 'inventory_close': 'passed', 'approval_execution': 'passed'}
