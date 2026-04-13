package contract

import "github.com/gin-gonic/gin"

// Module defines the contract every feature module must implement.
type Module interface {
	Register(r gin.IRouter)
}

type AuthMiddleware gin.HandlerFunc
type AdminMiddleware gin.HandlersChain
type RequestIDMiddleware gin.HandlerFunc
type RecoveryMiddleware gin.HandlerFunc
type LoggerMiddleware gin.HandlerFunc
