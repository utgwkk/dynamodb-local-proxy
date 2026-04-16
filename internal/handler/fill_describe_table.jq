{"ReadUnitsPerSecond":5, "Status":"ACTIVE", "WriteUnitsPerSecond":5} as $dummy_warm_throughput
| {"NumberOfDecreasesToday": 0, "ReadCapacityUnits": 0, "WriteCapacityUnits": 0} as $dummy_provisioned_throughput
| .Table.WarmThroughput //= $dummy_warm_throughput
| .Table.GlobalSecondaryIndexes[]?.WarmThroughput //= $dummy_warm_throughput
| .Table.ProvisionedThroughput //= $dummy_provisioned_throughput
