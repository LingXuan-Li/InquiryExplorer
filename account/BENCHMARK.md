# BENCHMARK

最終更新: 2026-11-27T16:16:47+09:00

| 関数                          | 呼び出し回数 |        平均 |       最大 |
| ----------------------------- | -----------: | ----------: | ---------: |
| `account.*Manager.Add`        |           24 | 58.993587ms | 360.9249ms |
| `account.*Manager.saveLocked` |           23 |  31.12986ms | 357.5958ms |
| `account.*Manager.Remove`     |            4 | 19.769825ms |  58.8865ms |
| `account.NewManager`          |           22 |  9.212309ms | 176.8781ms |
| `account.*Manager.Load`       |           26 |  8.917907ms | 176.8781ms |
| `account.*Manager.List`       |           35 |  8.881825ms |  79.1515ms |
| `account.*Manager.Save`       |            1 |    5.1367ms |   5.1367ms |
| `storage.SaveToken`           |           20 |  3.288085ms |  10.9049ms |
| `account.indexPath`           |           55 |  3.215503ms | 176.2693ms |
| `storage.LoadToken`           |            7 |  2.045414ms |  11.9833ms |
| `file.OAuthFolderPath`        |           93 |   1.90753ms | 176.2693ms |
| `storage.protectData`         |           20 |   1.44695ms |   6.4016ms |
| `storage.DeleteToken`         |            3 |   953.333µs |   1.1883ms |
| `storage.unprotectData`       |            2 |    862.95µs |   1.1474ms |
| `account.*Manager.Get`        |           26 |   585.719µs |   7.7384ms |
| `storage.tokenPath`           |           30 |      37.7µs |    596.4µs |
| `file.ConfigFolderPath`       |           93 |    12.161µs |    596.4µs |
| `storage.legacyTokenPath`     |            8 |          0s |         0s |
