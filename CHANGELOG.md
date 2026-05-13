# Changelog

## [0.4.1](https://github.com/riadshalaby/gohour/compare/v0.4.0...v0.4.1) (2026-05-13)


### ⚠ BREAKING CHANGES

* **delete:** simplify cleanup to interactive full-db-file deletion with Y confirmation

### Features

* add atwork mapper and billable rule flag ([54d64ee](https://github.com/riadshalaby/gohour/commit/54d64ee07848a0ca0dbc9aed990cea9ac5c5fcfd))
* **api:** expose GET /api/import/rule-match for file-pick prefill ([ad18b0b](https://github.com/riadshalaby/gohour/commit/ad18b0b72398bc0a925b5c12d363eec4e9a5dea1))
* **cli:** focus gohour on the web UI ([c312b6e](https://github.com/riadshalaby/gohour/commit/c312b6e670f13a1875f44f71931e4733c4ee443c))
* **config:** add interactive OnePoint-based epm rule creation command ([0d1f4ee](https://github.com/riadshalaby/gohour/commit/0d1f4ee0c5348ab8ffb02b49744b7082dc584d6d))
* **config:** store gohour data under ~/.gohour ([720a9a0](https://github.com/riadshalaby/gohour/commit/720a9a00983997f2a69ed136a83e8f9e494ae4d1))
* **config:** switch to onepoint.url home URL, nest import settings, and persist epm rule IDs ([45fcd2f](https://github.com/riadshalaby/gohour/commit/45fcd2fb707a6a1feabc8c4ae874ad924424549c))
* **delete:** simplify cleanup to interactive full-db-file deletion with Y confirmation ([3495055](https://github.com/riadshalaby/gohour/commit/349505515fbcf488e709d626001fa92116523325))
* **e2e:** add playwright smoke suite ([ab146af](https://github.com/riadshalaby/gohour/commit/ab146af0d92b6e774414b20d1a609c3f1cafe994))
* finalize v0.2.2 release changes ([5c2f6c1](https://github.com/riadshalaby/gohour/commit/5c2f6c17579ed46d3b43c523837a8b15ed656aee))
* implement serve web UI with cached views and shared classify logic ([2bc15c6](https://github.com/riadshalaby/gohour/commit/2bc15c6853ef4e58a89880f6d93b34b6a0e71b0c))
* improve auth resilience and web workflow UX ([c94533a](https://github.com/riadshalaby/gohour/commit/c94533a5e5a602a988b5d11423271a9875913c71))
* migrate module path for go install support ([b5aaae5](https://github.com/riadshalaby/gohour/commit/b5aaae539b4a02b9e2b7f53f3a8e54bec1ab6449))
* **rules:** drive mapper selection by file_template rule and add interactive mapper choice in config rule add ([bfc2023](https://github.com/riadshalaby/gohour/commit/bfc2023814fd8c79980e3d84d3520f6d941ecaf0))
* **submit:** add OnePoint worklog submission from SQLite with dry-run, ID resolution, and day-merge persist ([6d63681](https://github.com/riadshalaby/gohour/commit/6d636814861c5b71b60a6c404ed63533ad1997fd))
* **submit:** extend dry-run to print detailed per-entry payload preview ([a723efc](https://github.com/riadshalaby/gohour/commit/a723efcc0c6bfa012a3fea76966d65de48cffbda))
* **web:** add top-level Import file button to the month view header ([8c886aa](https://github.com/riadshalaby/gohour/commit/8c886aad3cab1109e6a6a952cef79b82e0cf280d))
* **web:** manage config rules in the web UI ([8ab9013](https://github.com/riadshalaby/gohour/commit/8ab901316f3283d23817247c9ae858ddecac7877))
* **web:** match import rules in the web UI ([167bea6](https://github.com/riadshalaby/gohour/commit/167bea62c80404c3ed3090e634fd63525861f940))
* **web:** pre-fill import dialog from matched rule on file selection ([d99540e](https://github.com/riadshalaby/gohour/commit/d99540e42ae61541675148bf4ee9b449b8a05e64))
* **web:** reconcile EPM imports automatically ([562a4eb](https://github.com/riadshalaby/gohour/commit/562a4ebe3b8440d70a116357bd7844b0d7ed02f4))
* **web:** ship v0.2.3 refresh/submit/import reliability and release tooling ([a19d0ba](https://github.com/riadshalaby/gohour/commit/a19d0ba98b06bab406d54d8d9370abb7e9b40824))


### Bug Fixes

* **ai:** fixed a script error ([dafd25f](https://github.com/riadshalaby/gohour/commit/dafd25f8b0dbdf582fefa94c81111b703d037ba0))
* clear all month days on remote delete ([889c120](https://github.com/riadshalaby/gohour/commit/889c12045e9765f45185f342cc1781196f05a685))
* **config:** validate mapper names against supported set ([1fde084](https://github.com/riadshalaby/gohour/commit/1fde08415235c45f57cb40a134a1ed5cfba249c0))
* **config:** write config files with owner-only permissions ([4b7f92a](https://github.com/riadshalaby/gohour/commit/4b7f92ac8f702adfdbbf02a6ebe795501334be6d))
* dependency updates ([0d798c6](https://github.com/riadshalaby/gohour/commit/0d798c61e1f051c7966f96f7cbffc3249df67e5a))
* **importer:** fail epm mapping when computed entry crosses midnight ([6f30b5a](https://github.com/riadshalaby/gohour/commit/6f30b5acc3f4414a28c4d39285fa7782f991a9e1))
* **importer:** parse generic billable override as minutes ([05247f2](https://github.com/riadshalaby/gohour/commit/05247f20ee932ce146d714fd4c5a77294bc50ff8))
* **importer:** validate epm day start/end ordering ([c9d5ca8](https://github.com/riadshalaby/gohour/commit/c9d5ca82384ef10e683cd5a491e41f3cad431a34))
* **onepoint:** deduplicate worklogs by time and IDs only ([794312d](https://github.com/riadshalaby/gohour/commit/794312de661cf204975d60fb51405d05091b0dde))
* **onepoint:** exclude locked existing worklogs from merge payload ([44b70ae](https://github.com/riadshalaby/gohour/commit/44b70aedb28c777a65ae90219340317797959d2d))
* **reconcile:** count all overlaps in conflict metrics ([13a3a97](https://github.com/riadshalaby/gohour/commit/13a3a97810f083d54885c7f6df870f1c8ed11839))
* **reconcile:** skip epm shifts that cross day boundaries ([b34c722](https://github.com/riadshalaby/gohour/commit/b34c722b2cf262ad0ea38082eb39c4d3c7f4be76))
* **submit:** preserve explicit non-billable worklogs ([6befbc3](https://github.com/riadshalaby/gohour/commit/6befbc3559b20b885416be7d83a6524aa3de21f3))
* **submit:** skip fully locked OnePoint days ([4c871de](https://github.com/riadshalaby/gohour/commit/4c871ded58c1e7af4b60dd32ff1a9f6080a048aa))
* **submit:** validate empty name tuples before id resolution ([1bee237](https://github.com/riadshalaby/gohour/commit/1bee2379632045a0f4ecf4c11dd9b5d2c5cb1bd3))
* **web:** remove confusing "Auto" billable option from import dialog ([ec6cd7d](https://github.com/riadshalaby/gohour/commit/ec6cd7d34b3c053605719dcce655e3b089847608))
* **web:** surface "Update matched rule" affordance on field override ([254468c](https://github.com/riadshalaby/gohour/commit/254468c8be07e19354dce99fea7437f78c3ba5d9))


### Chore

* **ai:** close cycle ([b96735f](https://github.com/riadshalaby/gohour/commit/b96735f5be846b4907899470ea5f13dc0b865066))
* pin next release version to 0.4.1 ([f8e7a61](https://github.com/riadshalaby/gohour/commit/f8e7a61b15e3b114d5fb5d5f0704b42b925be3e5))
* reset release state for clean 0.4.1 re-release ([917c2fa](https://github.com/riadshalaby/gohour/commit/917c2fae079bc95babb0ff9259ab7dee6fb4d131))

## [0.4.0](https://github.com/riadshalaby/gohour/compare/v0.4.0...v0.4.0) (2026-05-13)


### ⚠ BREAKING CHANGES

* **delete:** simplify cleanup to interactive full-db-file deletion with Y confirmation

### Features

* add atwork mapper and billable rule flag ([54d64ee](https://github.com/riadshalaby/gohour/commit/54d64ee07848a0ca0dbc9aed990cea9ac5c5fcfd))
* **api:** expose GET /api/import/rule-match for file-pick prefill ([ad18b0b](https://github.com/riadshalaby/gohour/commit/ad18b0b72398bc0a925b5c12d363eec4e9a5dea1))
* **cli:** focus gohour on the web UI ([c312b6e](https://github.com/riadshalaby/gohour/commit/c312b6e670f13a1875f44f71931e4733c4ee443c))
* **config:** add interactive OnePoint-based epm rule creation command ([0d1f4ee](https://github.com/riadshalaby/gohour/commit/0d1f4ee0c5348ab8ffb02b49744b7082dc584d6d))
* **config:** store gohour data under ~/.gohour ([720a9a0](https://github.com/riadshalaby/gohour/commit/720a9a00983997f2a69ed136a83e8f9e494ae4d1))
* **config:** switch to onepoint.url home URL, nest import settings, and persist epm rule IDs ([45fcd2f](https://github.com/riadshalaby/gohour/commit/45fcd2fb707a6a1feabc8c4ae874ad924424549c))
* **delete:** simplify cleanup to interactive full-db-file deletion with Y confirmation ([3495055](https://github.com/riadshalaby/gohour/commit/349505515fbcf488e709d626001fa92116523325))
* **e2e:** add playwright smoke suite ([ab146af](https://github.com/riadshalaby/gohour/commit/ab146af0d92b6e774414b20d1a609c3f1cafe994))
* finalize v0.2.2 release changes ([5c2f6c1](https://github.com/riadshalaby/gohour/commit/5c2f6c17579ed46d3b43c523837a8b15ed656aee))
* implement serve web UI with cached views and shared classify logic ([2bc15c6](https://github.com/riadshalaby/gohour/commit/2bc15c6853ef4e58a89880f6d93b34b6a0e71b0c))
* improve auth resilience and web workflow UX ([c94533a](https://github.com/riadshalaby/gohour/commit/c94533a5e5a602a988b5d11423271a9875913c71))
* migrate module path for go install support ([b5aaae5](https://github.com/riadshalaby/gohour/commit/b5aaae539b4a02b9e2b7f53f3a8e54bec1ab6449))
* **rules:** drive mapper selection by file_template rule and add interactive mapper choice in config rule add ([bfc2023](https://github.com/riadshalaby/gohour/commit/bfc2023814fd8c79980e3d84d3520f6d941ecaf0))
* **submit:** add OnePoint worklog submission from SQLite with dry-run, ID resolution, and day-merge persist ([6d63681](https://github.com/riadshalaby/gohour/commit/6d636814861c5b71b60a6c404ed63533ad1997fd))
* **submit:** extend dry-run to print detailed per-entry payload preview ([a723efc](https://github.com/riadshalaby/gohour/commit/a723efcc0c6bfa012a3fea76966d65de48cffbda))
* **web:** add top-level Import file button to the month view header ([8c886aa](https://github.com/riadshalaby/gohour/commit/8c886aad3cab1109e6a6a952cef79b82e0cf280d))
* **web:** manage config rules in the web UI ([8ab9013](https://github.com/riadshalaby/gohour/commit/8ab901316f3283d23817247c9ae858ddecac7877))
* **web:** match import rules in the web UI ([167bea6](https://github.com/riadshalaby/gohour/commit/167bea62c80404c3ed3090e634fd63525861f940))
* **web:** pre-fill import dialog from matched rule on file selection ([d99540e](https://github.com/riadshalaby/gohour/commit/d99540e42ae61541675148bf4ee9b449b8a05e64))
* **web:** reconcile EPM imports automatically ([562a4eb](https://github.com/riadshalaby/gohour/commit/562a4ebe3b8440d70a116357bd7844b0d7ed02f4))
* **web:** ship v0.2.3 refresh/submit/import reliability and release tooling ([a19d0ba](https://github.com/riadshalaby/gohour/commit/a19d0ba98b06bab406d54d8d9370abb7e9b40824))


### Bug Fixes

* **ai:** fixed a script error ([dafd25f](https://github.com/riadshalaby/gohour/commit/dafd25f8b0dbdf582fefa94c81111b703d037ba0))
* clear all month days on remote delete ([889c120](https://github.com/riadshalaby/gohour/commit/889c12045e9765f45185f342cc1781196f05a685))
* **config:** validate mapper names against supported set ([1fde084](https://github.com/riadshalaby/gohour/commit/1fde08415235c45f57cb40a134a1ed5cfba249c0))
* **config:** write config files with owner-only permissions ([4b7f92a](https://github.com/riadshalaby/gohour/commit/4b7f92ac8f702adfdbbf02a6ebe795501334be6d))
* dependency updates ([0d798c6](https://github.com/riadshalaby/gohour/commit/0d798c61e1f051c7966f96f7cbffc3249df67e5a))
* **importer:** fail epm mapping when computed entry crosses midnight ([6f30b5a](https://github.com/riadshalaby/gohour/commit/6f30b5acc3f4414a28c4d39285fa7782f991a9e1))
* **importer:** parse generic billable override as minutes ([05247f2](https://github.com/riadshalaby/gohour/commit/05247f20ee932ce146d714fd4c5a77294bc50ff8))
* **importer:** validate epm day start/end ordering ([c9d5ca8](https://github.com/riadshalaby/gohour/commit/c9d5ca82384ef10e683cd5a491e41f3cad431a34))
* **onepoint:** deduplicate worklogs by time and IDs only ([794312d](https://github.com/riadshalaby/gohour/commit/794312de661cf204975d60fb51405d05091b0dde))
* **onepoint:** exclude locked existing worklogs from merge payload ([44b70ae](https://github.com/riadshalaby/gohour/commit/44b70aedb28c777a65ae90219340317797959d2d))
* **reconcile:** count all overlaps in conflict metrics ([13a3a97](https://github.com/riadshalaby/gohour/commit/13a3a97810f083d54885c7f6df870f1c8ed11839))
* **reconcile:** skip epm shifts that cross day boundaries ([b34c722](https://github.com/riadshalaby/gohour/commit/b34c722b2cf262ad0ea38082eb39c4d3c7f4be76))
* **submit:** preserve explicit non-billable worklogs ([6befbc3](https://github.com/riadshalaby/gohour/commit/6befbc3559b20b885416be7d83a6524aa3de21f3))
* **submit:** skip fully locked OnePoint days ([4c871de](https://github.com/riadshalaby/gohour/commit/4c871ded58c1e7af4b60dd32ff1a9f6080a048aa))
* **submit:** validate empty name tuples before id resolution ([1bee237](https://github.com/riadshalaby/gohour/commit/1bee2379632045a0f4ecf4c11dd9b5d2c5cb1bd3))
* **web:** remove confusing "Auto" billable option from import dialog ([ec6cd7d](https://github.com/riadshalaby/gohour/commit/ec6cd7d34b3c053605719dcce655e3b089847608))
* **web:** surface "Update matched rule" affordance on field override ([254468c](https://github.com/riadshalaby/gohour/commit/254468c8be07e19354dce99fea7437f78c3ba5d9))


### Chore

* **ai:** close cycle ([b96735f](https://github.com/riadshalaby/gohour/commit/b96735f5be846b4907899470ea5f13dc0b865066))

## [0.4.0](https://github.com/riadshalaby/gohour/compare/v0.4.0...v0.4.0) (2026-05-13)


### ⚠ BREAKING CHANGES

* **delete:** simplify cleanup to interactive full-db-file deletion with Y confirmation

### Features

* add atwork mapper and billable rule flag ([54d64ee](https://github.com/riadshalaby/gohour/commit/54d64ee07848a0ca0dbc9aed990cea9ac5c5fcfd))
* **api:** expose GET /api/import/rule-match for file-pick prefill ([ad18b0b](https://github.com/riadshalaby/gohour/commit/ad18b0b72398bc0a925b5c12d363eec4e9a5dea1))
* **cli:** focus gohour on the web UI ([c312b6e](https://github.com/riadshalaby/gohour/commit/c312b6e670f13a1875f44f71931e4733c4ee443c))
* **config:** add interactive OnePoint-based epm rule creation command ([0d1f4ee](https://github.com/riadshalaby/gohour/commit/0d1f4ee0c5348ab8ffb02b49744b7082dc584d6d))
* **config:** store gohour data under ~/.gohour ([720a9a0](https://github.com/riadshalaby/gohour/commit/720a9a00983997f2a69ed136a83e8f9e494ae4d1))
* **config:** switch to onepoint.url home URL, nest import settings, and persist epm rule IDs ([45fcd2f](https://github.com/riadshalaby/gohour/commit/45fcd2fb707a6a1feabc8c4ae874ad924424549c))
* **delete:** simplify cleanup to interactive full-db-file deletion with Y confirmation ([3495055](https://github.com/riadshalaby/gohour/commit/349505515fbcf488e709d626001fa92116523325))
* **e2e:** add playwright smoke suite ([ab146af](https://github.com/riadshalaby/gohour/commit/ab146af0d92b6e774414b20d1a609c3f1cafe994))
* finalize v0.2.2 release changes ([5c2f6c1](https://github.com/riadshalaby/gohour/commit/5c2f6c17579ed46d3b43c523837a8b15ed656aee))
* implement serve web UI with cached views and shared classify logic ([2bc15c6](https://github.com/riadshalaby/gohour/commit/2bc15c6853ef4e58a89880f6d93b34b6a0e71b0c))
* improve auth resilience and web workflow UX ([c94533a](https://github.com/riadshalaby/gohour/commit/c94533a5e5a602a988b5d11423271a9875913c71))
* migrate module path for go install support ([b5aaae5](https://github.com/riadshalaby/gohour/commit/b5aaae539b4a02b9e2b7f53f3a8e54bec1ab6449))
* **rules:** drive mapper selection by file_template rule and add interactive mapper choice in config rule add ([bfc2023](https://github.com/riadshalaby/gohour/commit/bfc2023814fd8c79980e3d84d3520f6d941ecaf0))
* **submit:** add OnePoint worklog submission from SQLite with dry-run, ID resolution, and day-merge persist ([6d63681](https://github.com/riadshalaby/gohour/commit/6d636814861c5b71b60a6c404ed63533ad1997fd))
* **submit:** extend dry-run to print detailed per-entry payload preview ([a723efc](https://github.com/riadshalaby/gohour/commit/a723efcc0c6bfa012a3fea76966d65de48cffbda))
* **web:** add top-level Import file button to the month view header ([8c886aa](https://github.com/riadshalaby/gohour/commit/8c886aad3cab1109e6a6a952cef79b82e0cf280d))
* **web:** manage config rules in the web UI ([8ab9013](https://github.com/riadshalaby/gohour/commit/8ab901316f3283d23817247c9ae858ddecac7877))
* **web:** match import rules in the web UI ([167bea6](https://github.com/riadshalaby/gohour/commit/167bea62c80404c3ed3090e634fd63525861f940))
* **web:** pre-fill import dialog from matched rule on file selection ([d99540e](https://github.com/riadshalaby/gohour/commit/d99540e42ae61541675148bf4ee9b449b8a05e64))
* **web:** reconcile EPM imports automatically ([562a4eb](https://github.com/riadshalaby/gohour/commit/562a4ebe3b8440d70a116357bd7844b0d7ed02f4))
* **web:** ship v0.2.3 refresh/submit/import reliability and release tooling ([a19d0ba](https://github.com/riadshalaby/gohour/commit/a19d0ba98b06bab406d54d8d9370abb7e9b40824))


### Bug Fixes

* **ai:** fixed a script error ([dafd25f](https://github.com/riadshalaby/gohour/commit/dafd25f8b0dbdf582fefa94c81111b703d037ba0))
* clear all month days on remote delete ([889c120](https://github.com/riadshalaby/gohour/commit/889c12045e9765f45185f342cc1781196f05a685))
* **config:** validate mapper names against supported set ([1fde084](https://github.com/riadshalaby/gohour/commit/1fde08415235c45f57cb40a134a1ed5cfba249c0))
* **config:** write config files with owner-only permissions ([4b7f92a](https://github.com/riadshalaby/gohour/commit/4b7f92ac8f702adfdbbf02a6ebe795501334be6d))
* dependency updates ([0d798c6](https://github.com/riadshalaby/gohour/commit/0d798c61e1f051c7966f96f7cbffc3249df67e5a))
* **importer:** fail epm mapping when computed entry crosses midnight ([6f30b5a](https://github.com/riadshalaby/gohour/commit/6f30b5acc3f4414a28c4d39285fa7782f991a9e1))
* **importer:** parse generic billable override as minutes ([05247f2](https://github.com/riadshalaby/gohour/commit/05247f20ee932ce146d714fd4c5a77294bc50ff8))
* **importer:** validate epm day start/end ordering ([c9d5ca8](https://github.com/riadshalaby/gohour/commit/c9d5ca82384ef10e683cd5a491e41f3cad431a34))
* **onepoint:** deduplicate worklogs by time and IDs only ([794312d](https://github.com/riadshalaby/gohour/commit/794312de661cf204975d60fb51405d05091b0dde))
* **onepoint:** exclude locked existing worklogs from merge payload ([44b70ae](https://github.com/riadshalaby/gohour/commit/44b70aedb28c777a65ae90219340317797959d2d))
* **reconcile:** count all overlaps in conflict metrics ([13a3a97](https://github.com/riadshalaby/gohour/commit/13a3a97810f083d54885c7f6df870f1c8ed11839))
* **reconcile:** skip epm shifts that cross day boundaries ([b34c722](https://github.com/riadshalaby/gohour/commit/b34c722b2cf262ad0ea38082eb39c4d3c7f4be76))
* **submit:** preserve explicit non-billable worklogs ([6befbc3](https://github.com/riadshalaby/gohour/commit/6befbc3559b20b885416be7d83a6524aa3de21f3))
* **submit:** skip fully locked OnePoint days ([4c871de](https://github.com/riadshalaby/gohour/commit/4c871ded58c1e7af4b60dd32ff1a9f6080a048aa))
* **submit:** validate empty name tuples before id resolution ([1bee237](https://github.com/riadshalaby/gohour/commit/1bee2379632045a0f4ecf4c11dd9b5d2c5cb1bd3))
* **web:** remove confusing "Auto" billable option from import dialog ([ec6cd7d](https://github.com/riadshalaby/gohour/commit/ec6cd7d34b3c053605719dcce655e3b089847608))
* **web:** surface "Update matched rule" affordance on field override ([254468c](https://github.com/riadshalaby/gohour/commit/254468c8be07e19354dce99fea7437f78c3ba5d9))


### Chore

* **ai:** close cycle ([b96735f](https://github.com/riadshalaby/gohour/commit/b96735f5be846b4907899470ea5f13dc0b865066))
