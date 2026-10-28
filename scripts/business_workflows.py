"""Additional real HTTP business loops; called only by the owned disposable runner.

No remote URLs, database credentials or production instance switches are accepted.
Remaining flow groups are explicitly reported while this task is in progress.
"""

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
    return {'pdu': 'passed', 'stock_transfer': 'not_implemented_yet', 'inventory_close': 'not_implemented_yet', 'approval_execution': 'not_implemented_yet'}
