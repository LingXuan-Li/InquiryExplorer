# BENCHMARK

最終更新: 2026-11-27T16:16:47+09:00

| 関数                                 | 呼び出し回数 |       平均 |       最大 |
| ------------------------------------ | -----------: | ---------: | ---------: |
| `account.*Manager.Add`               |            5 | 67.19906ms |  263.703ms |
| `account.*Manager.saveLocked`        |            5 |  62.5149ms | 259.6122ms |
| `storage.SaveToken`                  |            7 | 4.367514ms |   9.1864ms |
| `drive.GetAllAccountsOverview`       |            4 |   4.1736ms |  13.7441ms |
| `drive.NewClient`                    |            5 |  3.22508ms |  12.5922ms |
| `drive.*persistentTokenSource.Token` |            6 | 2.828916ms |  13.3246ms |
| `storage.protectData`                |            7 | 1.644142ms |   5.0112ms |
| `storage.LoadToken`                  |            8 |   952.45µs |   1.7932ms |
| `storage.unprotectData`              |            5 |    807.5µs |   1.1981ms |
| `drive.ListFiles`                    |            1 |      569µs |      569µs |
| `drive.GetDriveOverview`             |            1 |      569µs |      569µs |
| `storage.DeleteToken`                |            2 |    565.4µs |    573.9µs |
| `account.NewManager`                 |            3 |  191.666µs |      575µs |
| `account.*Manager.Load`              |            3 |  191.666µs |      575µs |
| `storage.tokenPath`                  |           18 |   64.461µs |    589.4µs |
| `file.ConfigFolderPath`              |           32 |   36.259µs |    589.4µs |
| `file.OAuthFolderPath`               |           32 |   36.259µs |    589.4µs |
| `storage.legacyTokenPath`            |            6 |         0s |         0s |
| `account.*Manager.List`              |            3 |         0s |         0s |
| `cloud.GoogleConfig`                 |            3 |         0s |         0s |
| `account.indexPath`                  |            8 |         0s |         0s |
| `drive.sameToken`                    |           15 |         0s |         0s |
