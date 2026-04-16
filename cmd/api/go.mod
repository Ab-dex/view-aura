module github.com/Ab-dex/view-aura/cmd/api

go 1.25.0

require (
	github.com/Ab-dex/view-aura/internal/app v0.0.0
	github.com/Ab-dex/view-aura/internal/contract v0.0.0
	github.com/Ab-dex/view-aura/internal/modules/moderation v0.0.0
	github.com/Ab-dex/view-aura/internal/modules/movie v0.0.0
	github.com/Ab-dex/view-aura/internal/modules/notification v0.0.0
	github.com/Ab-dex/view-aura/internal/modules/payment v0.0.0
	github.com/Ab-dex/view-aura/internal/modules/profile v0.0.0 // indirect
	github.com/Ab-dex/view-aura/internal/modules/quota v0.0.0
	github.com/Ab-dex/view-aura/internal/modules/rating v0.0.0
	github.com/Ab-dex/view-aura/internal/modules/review v0.0.0
	github.com/Ab-dex/view-aura/internal/modules/social v0.0.0
	github.com/Ab-dex/view-aura/internal/modules/upload v0.0.0
	github.com/Ab-dex/view-aura/internal/modules/user v0.0.0
	github.com/Ab-dex/view-aura/internal/modules/watchlist v0.0.0
	github.com/Ab-dex/view-aura/internal/platform v0.0.0
	github.com/gin-gonic/gin v1.12.0 // indirect
	github.com/google/wire v0.7.0
	github.com/rs/zerolog v1.35.0
)

replace (
	github.com/Ab-dex/view-aura/internal/modules/moderation => ../../internal/modules/moderation
	github.com/Ab-dex/view-aura/internal/modules/movie => ../../internal/modules/movie
	github.com/Ab-dex/view-aura/internal/modules/notification => ../../internal/modules/notification
	github.com/Ab-dex/view-aura/internal/modules/payment => ../../internal/modules/payment
	github.com/Ab-dex/view-aura/internal/modules/profile => ../../internal/modules/profile
	github.com/Ab-dex/view-aura/internal/modules/quota => ../../internal/modules/quota
	github.com/Ab-dex/view-aura/internal/modules/rating => ../../internal/modules/rating
	github.com/Ab-dex/view-aura/internal/modules/review => ../../internal/modules/review
	github.com/Ab-dex/view-aura/internal/modules/social => ../../internal/modules/social
	github.com/Ab-dex/view-aura/internal/modules/upload => ../../internal/modules/upload
	github.com/Ab-dex/view-aura/internal/modules/user => ../../internal/modules/user
	github.com/Ab-dex/view-aura/internal/modules/watchlist => ../../internal/modules/watchlist
)

require (
	github.com/Ab-dex/view-aura/internal/events v0.0.0
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/sys v0.43.0 // indirect
)

require (
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/aws/aws-sdk-go-v2 v1.41.5 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.8 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.14 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.21 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.21 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.22 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/s3 v1.99.0 // indirect
	github.com/aws/smithy-go v1.24.2 // indirect
	github.com/bytedance/gopkg v0.1.4 // indirect
	github.com/bytedance/sonic v1.15.0 // indirect
	github.com/bytedance/sonic/loader v0.5.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.6 // indirect
	github.com/confluentinc/confluent-kafka-go/v2 v2.14.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/gin-contrib/sse v1.1.1 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/spec v0.20.4 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.30.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.3.2 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.28.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.9.1 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/nexus-rpc/sdk-go v0.6.0 // indirect
	github.com/pelletier/go-toml/v2 v2.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.59.0 // indirect
	github.com/redis/go-redis/v9 v9.18.0 // indirect
	github.com/robfig/cron v1.2.0 // indirect
	github.com/sagikazarmark/locafero v0.11.0 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/spf13/viper v1.21.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/stripe/stripe-go/v76 v76.25.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/swaggo/files v1.0.1 // indirect
	github.com/swaggo/gin-swagger v1.6.1 // indirect
	github.com/swaggo/swag v1.8.12 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.1 // indirect
	go.mongodb.org/mongo-driver/v2 v2.5.0 // indirect
	go.temporal.io/api v1.62.7 // indirect
	go.temporal.io/sdk v1.42.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/arch v0.25.0 // indirect
	golang.org/x/crypto v0.50.0 // indirect
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	golang.org/x/time v0.12.0 // indirect
	golang.org/x/tools v0.43.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260401024825-9d38bb4040a9 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260401024825-9d38bb4040a9 // indirect
	google.golang.org/grpc v1.80.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/Ab-dex/view-aura/internal/app => ../../internal/app

replace github.com/Ab-dex/view-aura/internal/platform => ../../internal/platform

replace github.com/Ab-dex/view-aura/internal/contract => ../../internal/contract

replace github.com/Ab-dex/view-aura/internal/events => ../../internal/events
