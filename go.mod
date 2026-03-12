module github.com/sagan/goaider

go 1.26

// Based on https://github.com/fengjiongmax/comfy2go
// Mod to fix some problems. See @mod .
replace github.com/richinsley/comfy2go => ./comfy2go

// fix-paste-delay and other mods
// replace github.com/elk-language/go-prompt => ../go-prompt
replace github.com/elk-language/go-prompt => github.com/sagan/go-prompt v0.0.0-20260312090041-ecf2c08e0777

require (
	cloud.google.com/go/translate v1.12.7
	github.com/JLugagne/jsonschema-infer v0.3.0
	github.com/disintegration/imaging v1.6.2
	github.com/dop251/goja v0.0.0-20260226184354-913bd86fb70c
	github.com/dop251/goja_nodejs v0.0.0-20260212111938-1f56ff5bcf14
	github.com/dsoprea/go-exif/v3 v3.0.1
	github.com/ebitengine/oto/v3 v3.4.0
	github.com/elk-language/go-prompt v1.3.1
	github.com/go-sprout/sprout v1.0.3
	github.com/gobwas/glob v0.2.3
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/hajimehoshi/go-mp3 v0.3.4
	github.com/invopop/jsonschema v0.13.0
	github.com/jmoiron/sqlx v1.4.0
	github.com/kaptinlin/jsonschema v0.7.5
	github.com/mattn/go-runewidth v0.0.20
	github.com/mithrandie/csvq-driver v1.7.0
	github.com/muesli/smartcrop v0.3.0
	github.com/natefinch/atomic v1.0.1
	github.com/pelletier/go-toml/v2 v2.2.4
	github.com/richinsley/comfy2go v0.7.1
	github.com/rwcarlsen/goexif v0.0.0-20190401172101-9e8deecbddbd
	github.com/sagan/zip v0.0.0-20240708090818-02b11188bf71
	github.com/saintfish/chardet v0.0.0-20230101081208-5e3ef4b5456d
	github.com/sirupsen/logrus v1.9.4
	github.com/spf13/cobra v1.10.2
	github.com/vincent-petithory/dataurl v1.0.0
	github.com/wujunwei928/edge-tts-go v0.0.0-20250315123430-d4675babeb96
	github.com/xuri/excelize/v2 v2.10.1
	github.com/xxr3376/gtboard v0.0.2
	golang.design/x/clipboard v0.7.1
	golang.org/x/exp v0.0.0-20260218203240-3dfff04db8fa
	golang.org/x/image v0.36.0
	golang.org/x/sync v0.19.0
	golang.org/x/sys v0.41.0
	golang.org/x/term v0.40.0
	golang.org/x/text v0.34.0
	golift.io/xtractr v0.3.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/auth v0.18.2 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	dario.cat/mergo v1.0.2 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/andybalholm/brotli v1.2.0 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/bodgit/plumbing v1.3.0 // indirect
	github.com/bodgit/sevenzip v1.6.1 // indirect
	github.com/bodgit/windows v1.0.1 // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/cavaliergopher/cpio v1.0.1 // indirect
	github.com/cavaliergopher/rpm v1.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dlclark/regexp2 v1.11.5 // indirect
	github.com/dsoprea/go-logging v0.0.0-20200710184922-b02d349568dd // indirect
	github.com/dsoprea/go-utility/v2 v2.0.0-20221003172846-a3e1774ef349 // indirect
	github.com/ebitengine/purego v0.10.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-errors/errors v1.5.1 // indirect
	github.com/go-json-experiment/json v0.0.0-20260214004413-d219187c3433 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-resty/resty/v2 v2.17.2 // indirect
	github.com/go-sourcemap/sourcemap v2.1.4+incompatible // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/golang/geo v0.0.0-20260129164528-943061e2742c // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/pprof v0.0.0-20260302011040-a15ffb7f9dcc // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.12 // indirect
	github.com/googleapis/gax-go/v2 v2.17.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kaptinlin/go-i18n v0.2.12 // indirect
	github.com/kaptinlin/jsonpointer v0.4.17 // indirect
	github.com/kaptinlin/messageformat-go v0.4.18 // indirect
	github.com/kdomanski/iso9660 v0.4.0 // indirect
	github.com/klauspost/compress v1.18.4 // indirect
	github.com/mailru/easyjson v0.9.1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-tty v0.0.7 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/mithrandie/csvq v1.18.1 // indirect
	github.com/mithrandie/go-file/v2 v2.1.0 // indirect
	github.com/mithrandie/go-text v1.6.0 // indirect
	github.com/mithrandie/ternary v1.1.1 // indirect
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646 // indirect
	github.com/nwaples/rardecode/v2 v2.2.2 // indirect
	github.com/peterebden/ar v0.0.0-20241106141004-20dc11b778e8 // indirect
	github.com/pierrec/lz4/v4 v4.1.25 // indirect
	github.com/pkg/term v1.2.0-beta.2 // indirect
	github.com/richardlehane/mscfb v1.0.6 // indirect
	github.com/richardlehane/msoleps v1.0.6 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/ryszard/tfutils v0.0.0-20161028141955-98de232c7c68 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/sshaman1101/dcompress v0.0.0-20200109162717-50436a6332de // indirect
	github.com/therootcompany/xz v1.0.1 // indirect
	github.com/tiendc/go-deepcopy v1.7.2 // indirect
	github.com/ulikunitz/xz v0.5.15 // indirect
	github.com/wk8/go-ordered-map/v2 v2.1.8 // indirect
	github.com/xuri/efp v0.0.1 // indirect
	github.com/xuri/nfp v0.0.2-0.20250530014748-2ddeb826f9a9 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.65.0 // indirect
	go.opentelemetry.io/otel v1.40.0 // indirect
	go.opentelemetry.io/otel/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/trace v1.40.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	go4.org v0.0.0-20260112195520-a5071408f32f // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/exp/shiny v0.0.0-20260218203240-3dfff04db8fa // indirect
	golang.org/x/mobile v0.0.0-20260217195705-b56b3793a9c4 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/oauth2 v0.35.0 // indirect
	google.golang.org/api v0.269.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260226221140-a57be14db171 // indirect
	google.golang.org/grpc v1.79.1 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
