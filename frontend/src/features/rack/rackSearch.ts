import type { RackDevicePosition } from '@/types/device'
import type { Rack } from '@/types/resource'

export interface RackSearchLayout {
  positions?: RackDevicePosition[]
}

export interface RackSearchTarget {
  rack: Rack
  position?: RackDevicePosition
}

function normalize(value: unknown) {
  return String(value ?? '').trim().toLocaleLowerCase()
}

export function rackSearchableText(rack: Rack, positions: RackDevicePosition[] = []) {
  const rackFields = [
    rack.code,
    rack.name,
    rack.row,
    rack.column,
    rack.aisle,
    rack.zone,
    rack.manufacturer,
    rack.modelNumber,
    rack.manager,
    rack.purpose,
  ]
  const deviceFields = positions.flatMap((position) => [
    position.device?.name,
    position.device?.code,
    position.device?.serialNumber,
  ])
  return [...rackFields, ...deviceFields].filter(Boolean).join(' ').toLocaleLowerCase()
}

/**
 * Finds a device first (exact name before partial name), then falls back to a rack.
 * The returned position is used by the screen to select and scroll to the U slot.
 */
export function findRackSearchTarget(
  racks: Rack[],
  layouts: Record<string, RackSearchLayout | undefined>,
  keyword: string,
): RackSearchTarget | null {
  const normalizedKeyword = normalize(keyword)
  if (!normalizedKeyword) return null

  const deviceMatches = racks.flatMap((rack) => {
    const positions = layouts[rack.id]?.positions ?? []
    return positions
      .filter((position) => normalize(position.device?.name).includes(normalizedKeyword))
      .map((position) => ({ rack, position, exact: normalize(position.device?.name) === normalizedKeyword }))
  })
  const deviceMatch = deviceMatches.find((item) => item.exact) ?? deviceMatches[0]
  if (deviceMatch) return { rack: deviceMatch.rack, position: deviceMatch.position }

  const rack = racks.find((item) => rackSearchableText(item, layouts[item.id]?.positions ?? []).includes(normalizedKeyword))
  return rack ? { rack } : null
}
