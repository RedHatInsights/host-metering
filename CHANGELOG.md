# [1.3.0](https://github.com/RedHatInsights/host-metering/compare/v1.2.0...v1.3.0) (2024-05-27)


### Features

* add optional InstanceID field to log output ([#46](https://github.com/RedHatInsights/host-metering/issues/46)) ([27751a7](https://github.com/RedHatInsights/host-metering/commit/27751a736224d7e8bcae8bec58cba2ed95e00720))
* enable host-metering.service on rpm installation ([d231460](https://github.com/RedHatInsights/host-metering/commit/d23146027411a881a6f8e5cfdd6a87513ec69fba))

# [1.2.0](https://github.com/RedHatInsights/host-metering/compare/v1.1.0...v1.2.0) (2023-12-19)


### Bug Fixes

* `subscription-manager list --installed` call failure ([698ff05](https://github.com/RedHatInsights/host-metering/commit/698ff05326c44b41e794d72a6e9e6f30039d701c))
* log subscription-manager stdout on error ([aa9f13e](https://github.com/RedHatInsights/host-metering/commit/aa9f13e9bb9e2faaf49ed66664392a8bc496e73a))


### Features

* add send_hostname configuration option ([5c2ce20](https://github.com/RedHatInsights/host-metering/commit/5c2ce207ea826249ca6b588eb80d04b114fece7f))
* filter out labels based on configuration ([1a948ff](https://github.com/RedHatInsights/host-metering/commit/1a948ff0db5f7e7478ba4041399ff06284e488a7))
* send display_name label with host's name/fqdn ([5e0bc3c](https://github.com/RedHatInsights/host-metering/commit/5e0bc3cd837b856c8dcf1e35415abc491a9720ed))

# [1.1.0](https://github.com/RedHatInsights/host-metering/compare/v1.0.0...v1.1.0) (2023-12-13)


### Bug Fixes

* log file permissions ([e7a68ea](https://github.com/RedHatInsights/host-metering/commit/e7a68ea74bb1a9256851bc4d0ccf0a68de16d320))


### Features

* add label refresh on label_refresh_interval ([6cf0df3](https://github.com/RedHatInsights/host-metering/commit/6cf0df3506aee8819aa9e1abd4c7bb4f38c3e23e))
* add prometheus recording rule to aggregate measurements ([#22](https://github.com/RedHatInsights/host-metering/issues/22)) ([55dc8b8](https://github.com/RedHatInsights/host-metering/commit/55dc8b8c6b45b7cd42b236790740ba241731668b))
* capture and send installed products ID ([#25](https://github.com/RedHatInsights/host-metering/issues/25)) ([5f5e552](https://github.com/RedHatInsights/host-metering/commit/5f5e552abb5de46981fa9324341850b41eb1e937))
* configurable log prefix ([fcb8555](https://github.com/RedHatInsights/host-metering/commit/fcb8555b8e2946b481d6067e88ae006452578f29))
* document go proxy envs in man page ([773a12f](https://github.com/RedHatInsights/host-metering/commit/773a12fd4f6dfd3b76e53c1c1ad7e3a89c9e3f3d))
* shorten go proxy env description ([728e497](https://github.com/RedHatInsights/host-metering/commit/728e49749e84038ee2a5c83103607b41209ce047))

# 1.0.0 (2023-11-07)


### Bug Fixes

* add external_organization label ([92489c2](https://github.com/RedHatInsights/host-metering/commit/92489c2ecc8cc9f9c11075a2322e2987d45e9ea9))
* change prometheus remote write url ([f2131db](https://github.com/RedHatInsights/host-metering/commit/f2131db25a4f1c4112c28148547dc7324383367f))
* Count the number of vCPUs instead of the number of CPU cores ([7af0f82](https://github.com/RedHatInsights/host-metering/commit/7af0f8274a58e8e62c7d9c02185bb7e145d31818))
* crash on failed cert watcher creation ([b514d48](https://github.com/RedHatInsights/host-metering/commit/b514d48c70ba828c6f37d9ddb87d70fec7bff27a))
* crash when host cert is not available ([e48c1de](https://github.com/RedHatInsights/host-metering/commit/e48c1dee2c08f3b72677311cb921181f544f8e20))
* do not try to notify if required data are missing ([#14](https://github.com/RedHatInsights/host-metering/issues/14)) ([8620adf](https://github.com/RedHatInsights/host-metering/commit/8620adfc8c864943ded9e7a940aed8ec3036df75))
* don't print success on http errors ([1e3d917](https://github.com/RedHatInsights/host-metering/commit/1e3d9170489abc8314e14ac70d96c3bcbfb26254))
* Don't special-case first index in the metrics log ([4311117](https://github.com/RedHatInsights/host-metering/commit/431111759b4d6a3f117a67ab40687d3a579903b9))
* double call of `subscription-manager identity` ([#17](https://github.com/RedHatInsights/host-metering/issues/17)) ([e578ee9](https://github.com/RedHatInsights/host-metering/commit/e578ee95e7ef49fc3ea2e421835accbc09b0d92e))
* dropping of expired samples ([#19](https://github.com/RedHatInsights/host-metering/issues/19)) ([f2d17ed](https://github.com/RedHatInsights/host-metering/commit/f2d17ededc3e250e94ed4cc9744e387a9a343458))
* end of line from CRLF to LF of some files ([b552e01](https://github.com/RedHatInsights/host-metering/commit/b552e017daafe9e48a444132725889e59bc127cc))
* Fix the billing info generated for the GCP marketplace ([9e023b0](https://github.com/RedHatInsights/host-metering/commit/9e023b0c6dc178d7d48a405133ffdeebbe46f023))
* follow proxy settings defined in env vars ([83db7ba](https://github.com/RedHatInsights/host-metering/commit/83db7baec6f92ea951aa53455b0d17a8125ee8db))
* HostInfo not loaded in deamon mode ([e40c017](https://github.com/RedHatInsights/host-metering/commit/e40c01701dd8dae1745cb02dba102e1985b1c08b))
* Introduce the INIConfig type ([15453bf](https://github.com/RedHatInsights/host-metering/commit/15453bf0b297a4af4368a0f0db9512ea39fd2e44))
* Introduce the MultiError structure ([40548cd](https://github.com/RedHatInsights/host-metering/commit/40548cdafae8a89500b68c37499ffd7bd5a9fec5))
* Log the configuration messages by default ([6412a96](https://github.com/RedHatInsights/host-metering/commit/6412a96d840545a2a9a6c039bb9bf50511cabf63))
* logging improvements for easier debugging ([#18](https://github.com/RedHatInsights/host-metering/issues/18)) ([286b1e6](https://github.com/RedHatInsights/host-metering/commit/286b1e6e9e8bc3213b41d713c4c56674551f24a5))
* Move billing info to a new structure ([8e24825](https://github.com/RedHatInsights/host-metering/commit/8e248253fad2cb3ca418163ef6ad18bba288d8e7))
* move start of timers after initial notify ([0697014](https://github.com/RedHatInsights/host-metering/commit/0697014cb5161b8262575129cbdb01fa69ee250b))
* Prepare the metrics log for the introduction of checkpoints ([e94166b](https://github.com/RedHatInsights/host-metering/commit/e94166b0665b8d17e1772200760847c9eaae00ff))
* prevent overlapping remote writes ([0de885a](https://github.com/RedHatInsights/host-metering/commit/0de885a2b8ebcc861d5bf225baf23d1ed294f447))
* Print debug messages by default without further configuration ([89fde19](https://github.com/RedHatInsights/host-metering/commit/89fde19a6de034e547d6de9516ba7fcff0dd478b))
* Rename the cpu cache to the metrics log ([1d7bbf0](https://github.com/RedHatInsights/host-metering/commit/1d7bbf0ef72f048d8066a22a41418329aaf15f1a))
* return error when out of retries ([37d8670](https://github.com/RedHatInsights/host-metering/commit/37d8670d11109acf4f97fc45fe4db77ebc972ac6))
* selinux denials - dbus send msgs, squid port connect ([d440e73](https://github.com/RedHatInsights/host-metering/commit/d440e73b130f198f31d6276341bc43fbf4bd6f94))
* Simplify loading of the host info from facts ([32ce8d5](https://github.com/RedHatInsights/host-metering/commit/32ce8d5f60c024f9e94275c673b74ef24fd659f3))
* stop/restart on rpm uninstall/update, remove selinux policy on rpm uninstall ([#21](https://github.com/RedHatInsights/host-metering/issues/21)) ([7e025d1](https://github.com/RedHatInsights/host-metering/commit/7e025d1a58713d0d630b824e4816c0972ce6f858))
* The `log_path` configuration attribute should set the log path ([46c20e7](https://github.com/RedHatInsights/host-metering/commit/46c20e7e6da729ad4409db0b48693c438ec68a40))
* throw away samples on Prometheus Remote Write 4xx errors ([#15](https://github.com/RedHatInsights/host-metering/issues/15)) ([f92a91b](https://github.com/RedHatInsights/host-metering/commit/f92a91b8a9174ca3b7c72bafac4cb5ae7da98ac4))
* tune http Transport config ([bfb5b87](https://github.com/RedHatInsights/host-metering/commit/bfb5b8756027c798a962ae656cc043250ae87f6a))
* Unify parsing of uint configuration values ([b511bc4](https://github.com/RedHatInsights/host-metering/commit/b511bc4ed40c573e6fb13d90de2f93a38f79af09))
* Unify the configuration of the host certificates ([05be176](https://github.com/RedHatInsights/host-metering/commit/05be17692551acea817cffff660e0ada2872e4ef))
* Unify the execution of subscription-manager ([fff1554](https://github.com/RedHatInsights/host-metering/commit/fff15548f812120350e54000294afef8b16f79dc))
* Unify the processing of the subscription-manager outputs ([9901c6d](https://github.com/RedHatInsights/host-metering/commit/9901c6d6e44d46f2b1f2a145342f7fef8d0424a3))
* Use the subscription manager to get the host id ([52011f3](https://github.com/RedHatInsights/host-metering/commit/52011f3f0a4d7e64f7b38f8ea8a9ef38e5f3068f))


### Features

* add systemd service unit file ([d1123a0](https://github.com/RedHatInsights/host-metering/commit/d1123a024e565d6568fc5d26138b7714187467dc))
* add unit suffix to interval configuration variables ([63de6c8](https://github.com/RedHatInsights/host-metering/commit/63de6c80fe365e1de02d80b4c5a1f9d2c870893e))
* cache for CPU timeseries ([2219dca](https://github.com/RedHatInsights/host-metering/commit/2219dcadbec08f10fb9094facdf344e1ae624b5c))
* client daemon PoC ([7e4def3](https://github.com/RedHatInsights/host-metering/commit/7e4def37ae5257eee9c079da971fb7ad38431375))
* collect metrics/labels and notify right after daemon start ([fd50d7c](https://github.com/RedHatInsights/host-metering/commit/fd50d7cf92f4ed828b286388d989c80f1794ca54))
* **config:** loading from config file ([061de33](https://github.com/RedHatInsights/host-metering/commit/061de3304b42c5fcbe19c2eab3de57b59e646913))
* **config:** loading from environmetal variables ([0cc9d32](https://github.com/RedHatInsights/host-metering/commit/0cc9d3292cc32234eab9115120b29f4e395f4391))
* configurable Prometheus remote write timeout ([5fd555e](https://github.com/RedHatInsights/host-metering/commit/5fd555e219cd0b0f72b98aa8458f4040c088aeb9))
* Ensure exclusive access to the metrics log ([6cdd6f4](https://github.com/RedHatInsights/host-metering/commit/6cdd6f4b0ac49112439154f95ad9f63f9a8e2935))
* filter out samples older than maxAge ([743a349](https://github.com/RedHatInsights/host-metering/commit/743a3497f7d60c58e43caa51a58407ca00a5a5c5))
* **HostInfo:** load information via subscription-manager ([88ce8f4](https://github.com/RedHatInsights/host-metering/commit/88ce8f4b34d1d366e10db3369513b744f0e3ddf6))
* make CPU cache path configurable ([4dc8823](https://github.com/RedHatInsights/host-metering/commit/4dc8823d83f0013deb8fbf2b9b57f386f96789ca))
* metrics_max_age_sec config value ([4cd9102](https://github.com/RedHatInsights/host-metering/commit/4cd910247af33279b3ad65fdee16dfdf2206d206))
* monitor and react to subscription changes ([82473d3](https://github.com/RedHatInsights/host-metering/commit/82473d372a99ac1f904d409e65ec3072c1a2eba4))
* **notify:** recreate http client on host info change ([124c7f4](https://github.com/RedHatInsights/host-metering/commit/124c7f49e9d3abaa3ccc3f7b15420ce69c4339d0))
* print Host Info after it is reloaded ([30338e9](https://github.com/RedHatInsights/host-metering/commit/30338e9d596d241e40246a443d829b3527bdf52b))
* **prometheus:** incremental back-off ([1c3c2b3](https://github.com/RedHatInsights/host-metering/commit/1c3c2b30d72c7cc7083584f867e2fac0ca90dfd9))
* remove CLI options ([fa8a0a7](https://github.com/RedHatInsights/host-metering/commit/fa8a0a78f1b89479015ea46059a9270595e3d212))
* reuse http client for notification ([eb49b5e](https://github.com/RedHatInsights/host-metering/commit/eb49b5e43e5db1683c30693640dbf51e3dcf6637))
* selinux policy ([e460539](https://github.com/RedHatInsights/host-metering/commit/e4605391cc480326bf4d1ce042e0d7680b4c5514))
* send conversions_success label when host was converted ([#16](https://github.com/RedHatInsights/host-metering/issues/16)) ([fe8b644](https://github.com/RedHatInsights/host-metering/commit/fe8b644950766cf0c090335dceed43e05cf284c1))
* stop and re-run deamon ([09047be](https://github.com/RedHatInsights/host-metering/commit/09047bebbab06ecc9758bf9138db5635679ab845))
* use cpu cache ([eef9d25](https://github.com/RedHatInsights/host-metering/commit/eef9d25c2d0b8a013da1cc65e19cb88a4645097a))
* Use std logging library ([3ce70eb](https://github.com/RedHatInsights/host-metering/commit/3ce70eb6cfb44229b629a1d8bbd6326536e180c9))
* **write:** include extended HostInfo as labels ([74a9505](https://github.com/RedHatInsights/host-metering/commit/74a950535106d87e0a7fbce1197bdcd080e79fbf))
