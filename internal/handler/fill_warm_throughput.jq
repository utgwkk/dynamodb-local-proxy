{"ReadUnitsPerSecond":5, "Status":"ACTIVE", "WriteUnitsPerSecond":5} as $dummy_field
| .Table.WarmThroughput //= $dummy_field
| .Table.GlobalSecondaryIndexes[].WarmThroughput //= $dummy_field
