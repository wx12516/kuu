package rest

import (
	"github.com/gin-gonic/gin"
	"github.com/kuuland/kuu"
)

const (
	// BeforeCreateRouteEnum 新增前置
	BeforeCreateRouteEnum = iota
	// BeforeUpdateRouteEnum 更新前置
	BeforeUpdateRouteEnum
	// AfterUpdateRouteEnum 更新后置
	AfterUpdateRouteEnum
	// AfterCreateRouteEnum 新增后置
	AfterCreateRouteEnum
	// BeforeRemoveRouteEnum 删除前置
	BeforeRemoveRouteEnum
	// AfterRemoveRouteEnum 删除后置
	AfterRemoveRouteEnum
	// BeforeListRouteEnum 列表前置
	BeforeListRouteEnum
	// AfterListRouteEnum 列表后置
	AfterListRouteEnum
	// BeforeIDRouteEnum ID后置
	BeforeIDRouteEnum
	// AfterIDRouteEnum ID后置
	AfterIDRouteEnum
)

// Scope 钩子上下文实体
type Scope struct {
	Context      *gin.Context
	Cache        kuu.H
	Model        kuu.IModel
	Params       *Params
	CreateData   *[]kuu.H
	ResponseData *kuu.H
	RemoveCond   *kuu.H
	RemoveDoc    *kuu.H
	RemoveAll    bool
	UpdateCond   *kuu.H
	UpdateDoc    *kuu.H
	UpdateAll    bool
}

// CallMethod 调用钩子函数
func (scope *Scope) CallMethod(action int, schema *kuu.Schema) (err error) {
	switch action {
	case BeforeCreateRouteEnum:
		if s, ok := schema.Origin.(IBeforeCreateRoute); ok {
			err = s.BeforeCreateRoute(scope)
		}
	case AfterCreateRouteEnum:
		if s, ok := schema.Origin.(IAfterCreateRoute); ok {
			err = s.AfterCreateRoute(scope)
		}
	case BeforeUpdateRouteEnum:
		if s, ok := schema.Origin.(IBeforeUpdateRoute); ok {
			err = s.BeforeUpdateRoute(scope)
		}
	case AfterUpdateRouteEnum:
		if s, ok := schema.Origin.(IAfterUpdateRoute); ok {
			err = s.AfterUpdateRoute(scope)
		}
	case BeforeRemoveRouteEnum:
		if s, ok := schema.Origin.(IBeforeRemoveRoute); ok {
			err = s.BeforeRemoveRoute(scope)
		}
	case AfterRemoveRouteEnum:
		if s, ok := schema.Origin.(IAfterRemoveRoute); ok {
			err = s.AfterRemoveRoute(scope)
		}
	case BeforeListRouteEnum:
		if s, ok := schema.Origin.(IBeforeListRoute); ok {
			err = s.BeforeListRoute(scope)
		}
	case AfterListRouteEnum:
		if s, ok := schema.Origin.(IAfterListRoute); ok {
			err = s.AfterListRoute(scope)
		}
	case BeforeIDRouteEnum:
		if s, ok := schema.Origin.(IBeforeIDRoute); ok {
			err = s.BeforeIDRoute(scope)
		}
	case AfterIDRouteEnum:
		if s, ok := schema.Origin.(IAfterIDRoute); ok {
			err = s.AfterIDRoute(scope)
		}
	}
	return err
}
