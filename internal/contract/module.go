package contract

import "github.com/gin-gonic/gin"

type Module interface {
	Register(r gin.IRouter)
}

type AuthMiddleware gin.HandlerFunc
type AdminMiddleware gin.HandlersChain
type RequestIDMiddleware gin.HandlerFunc
type RecoveryMiddleware gin.HandlerFunc
type LoggerMiddleware gin.HandlerFunc

// Typed module wrappers so Wire can distinguish between them.
type AuthModule struct{ Module }
type UserModule struct{ Module }
type MovieModule struct{ Module }
type RatingModule struct{ Module }
type ReviewModule struct{ Module }
type WatchlistModule struct{ Module }
type SocialModule struct{ Module }
type NotificationModule struct{ Module }
type PaymentModule struct{ Module }
type UploadModule struct{ Module }
type QuotaModule struct{ Module }
type ModerationModule struct{ Module }
type ProfileModule struct{ Module }
