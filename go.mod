module github.com/sagan/goaider

go 1.25

// Based on https://github.com/fengjiongmax/comfy2go
// Mod to fix some problems. See @mod .
replace github.com/richinsley/comfy2go => ./comfy2go

require (
	cloud.google.com/go/translate v1.12.7
	github.com/c-bata/go-prompt v0.2.6
	github.com/disintegration/imaging v1.6.2
	github.com/dop251/goja v0.0.0-20251201205617-2bb4c724c0f9
	github.com/dop251/goja_nodejs v0.0.0-20251015164255-5e94316bedaf
	github.com/dsoprea/go-exif/v3 v3.0.1
	github.com/ebitengine/oto/v3 v3.4.0
	github.com/go-sprout/sprout v1.0.2
	github.com/gobwas/glob v0.2.3
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/hajimehoshi/go-mp3 v0.3.4
	github.com/invopop/jsonschema v0.13.0
	github.com/jmoiron/sqlx v1.4.0
	github.com/kaptinlin/jsonschema v0.6.2
	github.com/mattn/go-runewidth v0.0.19
	github.com/mithrandie/csvq-driver v1.7.0
	github.com/muesli/smartcrop v0.3.0
	github.com/natefinch/atomic v1.0.1
	github.com/pelletier/go-toml/v2 v2.2.4
	github.com/richinsley/comfy2go v0.6.6
	github.com/rwcarlsen/goexif v0.0.0-20190401172101-9e8deecbddbd
	github.com/sagan/zip v0.0.0-20240708090818-02b11188bf71
	github.com/saintfish/chardet v0.0.0-20230101081208-5e3ef4b5456d
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/cobra v1.10.1
	github.com/vincent-petithory/dataurl v1.0.0
	github.com/wujunwei928/edge-tts-go v0.0.0-20250315123430-d4675babeb96
	github.com/xuri/excelize/v2 v2.10.0
	github.com/xxr3376/gtboard v0.0.2
	golang.design/x/clipboard v0.7.1
	golang.org/x/exp v0.0.0-20251125195548-87e1e737ad39
	golang.org/x/image v0.34.0
	golang.org/x/sync v0.19.0
	golang.org/x/sys v0.39.0
	golang.org/x/term v0.37.0
	golang.org/x/text v0.32.0
	golift.io/xtractr v0.2.2
	gopkg.in/yaml.v3 v3.0.1
)

require (
	cloud.google.com/go v0.121.6 // indirect
	cloud.google.com/go/auth v0.16.4 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.8.0 // indirect
	dario.cat/mergo v1.0.2 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/andybalholm/brotli v1.0.5 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/bodgit/plumbing v1.3.0 // indirect
	github.com/bodgit/sevenzip v1.4.0 // indirect
	github.com/bodgit/windows v1.0.1 // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/clipperhouse/uax29/v2 v2.2.0 // indirect
	github.com/connesc/cipherio v0.2.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dlclark/regexp2 v1.11.4 // indirect
	github.com/dsoprea/go-logging v0.0.0-20200710184922-b02d349568dd // indirect
	github.com/dsoprea/go-utility/v2 v2.0.0-20221003172846-a3e1774ef349 // indirect
	github.com/ebitengine/purego v0.9.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-errors/errors v1.4.2 // indirect
	github.com/go-json-experiment/json v0.0.0-20251027170946-4849db3c2f7e // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-resty/resty/v2 v2.12.0 // indirect
	github.com/go-sourcemap/sourcemap v2.1.4+incompatible // indirect
	github.com/goccy/go-yaml v1.19.0 // indirect
	github.com/golang/geo v0.0.0-20210211234256-740aa86cb551 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/pprof v0.0.0-20240727154555-813a5fbdbec8 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/googleapis/gax-go/v2 v2.15.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kaptinlin/go-i18n v0.2.0 // indirect
	github.com/kaptinlin/jsonpointer v0.4.6 // indirect
	github.com/kaptinlin/messageformat-go v0.4.6 // indirect
	github.com/kdomanski/iso9660 v0.3.3 // indirect
	github.com/klauspost/compress v1.16.3 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/mattn/go-tty v0.0.3 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/mithrandie/csvq v1.18.1 // indirect
	github.com/mithrandie/go-file/v2 v2.1.0 // indirect
	github.com/mithrandie/go-text v1.6.0 // indirect
	github.com/mithrandie/ternary v1.1.1 // indirect
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646 // indirect
	github.com/nwaples/rardecode v1.1.3 // indirect
	github.com/pierrec/lz4/v4 v4.1.17 // indirect
	github.com/pkg/term v1.2.0-beta.2 // indirect
	github.com/richardlehane/mscfb v1.0.4 // indirect
	github.com/richardlehane/msoleps v1.0.4 // indirect
	github.com/ryszard/tfutils v0.0.0-20161028141955-98de232c7c68 // indirect
	github.com/spf13/cast v1.9.2 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/tiendc/go-deepcopy v1.7.1 // indirect
	github.com/ulikunitz/xz v0.5.11 // indirect
	github.com/wk8/go-ordered-map/v2 v2.1.8 // indirect
	github.com/xuri/efp v0.0.1 // indirect
	github.com/xuri/nfp v0.0.2-0.20250530014748-2ddeb826f9a9 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0 // indirect
	go.opentelemetry.io/otel v1.36.0 // indirect
	go.opentelemetry.io/otel/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/trace v1.36.0 // indirect
	go4.org v0.0.0-20230225012048-214862532bf5 // indirect
	golang.org/x/crypto v0.43.0 // indirect
	golang.org/x/exp/shiny v0.0.0-20250606033433-dcc06ee1d476 // indirect
	golang.org/x/mobile v0.0.0-20250606033058-a2a15c67f36f // indirect
	golang.org/x/net v0.46.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	google.golang.org/api v0.247.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250818200422-3122310a409c // indirect
	google.golang.org/grpc v1.74.2 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
