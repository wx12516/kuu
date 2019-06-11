package kuu

import (
	"errors"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"regexp"
	"strings"
)

// LoginHandlerFunc
type LoginHandlerFunc func(*Context) (jwt.MapClaims, uint, error)

var (
	TokenKey  = "Token"
	Whitelist = []interface{}{
		"GET /",
		"GET /favicon.ico",
		"GET /whitelist",
		"POST /login",
		"GET /enum",
		"GET /meta",
		regexp.MustCompile("GET /assets"),
	}
	ExpiresSeconds = 86400
	SignContextKey = "SignContext"
	loginHandler   = defaultLoginHandler
)

const (
	RedisSecretKey = "secret"
	RedisOrgKey    = "org"
)

// InWhitelist
func InWhitelist(c *gin.Context) bool {
	if len(Whitelist) == 0 {
		return false
	}
	input := strings.ToUpper(fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path))
	for _, item := range Whitelist {
		if v, ok := item.(string); ok {
			v = strings.ToUpper(v)
			prefix := C().GetString("prefix")
			if v == input {
				return true
			} else if C().DefaultGetBool("whitelist:prefix", true) && prefix != "" {
				old := strings.ToUpper(fmt.Sprintf("%s ", c.Request.Method))
				with := strings.ToUpper(fmt.Sprintf("%s %s", c.Request.Method, prefix))
				v = strings.Replace(v, old, with, 1)
				if v == input {
					return true
				}
			}
		} else if v, ok := item.(*regexp.Regexp); ok {
			if v.MatchString(input) {
				return true
			}
		}
	}
	return false
}

// AddWhitelist support string and *regexp.Regexp.
func AddWhitelist(rules ...interface{}) {
	Whitelist = append(Whitelist, rules...)
}

func saveHistory(secretData *SignSecret) {
	history := SignHistory{
		SecretID:   secretData.ID,
		SecretData: secretData.Secret,
		Token:      secretData.Token,
		Method:     secretData.Method,
	}
	DB().Create(&history)
}

// GenRedisKey
func RedisKeyBuilder(keys ...string) string {
	args := []string{RedisPrefix}
	for _, k := range keys {
		args = append(args, k)
	}
	return strings.Join(args, "_")
}

// ParseToken
var ParseToken = func(c *gin.Context) string {
	// querystring > header > cookie
	var token string
	token = c.Query(TokenKey)
	if token == "" {
		token = c.GetHeader(TokenKey)
		if token == "" {
			token = c.GetHeader("Authorization")
		}
		if token == "" {
			token = c.GetHeader("api_key")
		}
	}
	if token == "" {
		token, _ = c.Cookie(TokenKey)
	}
	return token
}

// DecodedContext
func DecodedContext(c *gin.Context) (*SignContext, error) {
	token := ParseToken(c)
	if token == "" {
		return nil, errors.New(L(c, "未找到令牌"))
	}
	data := SignContext{Token: token}
	// 解析UID
	var secret SignSecret
	if v, err := RedisClient.Get(RedisKeyBuilder(RedisSecretKey, token)).Result(); err == nil {
		Parse(v, &secret)
	} else {
		DB().Where(&SignSecret{Token: token}).Find(&secret)
	}
	data.UID = secret.UID
	// 解析OrgID
	var org SignOrg
	if v, err := RedisClient.Get(RedisKeyBuilder(RedisOrgKey, token)).Result(); err == nil {
		Parse(v, &org)
	} else {
		DB().Where(&SignOrg{Token: token}).Find(&org)
	}
	data.OrgID = org.OrgID
	// 验证令牌
	if secret.Secret == "" {
		return nil, errors.New(Lang(c, "secret_invalid", "Secret is invalid: {{uid}} {{token}}", gin.H{"uid": data.UID, "token": token}))
	}
	if secret.Method == "LOGOUT" {
		return nil, errors.New(Lang(c, "token_expired", "Token has expired: '{{token}}'", gin.H{"token": token}))
	}
	data.Secret = &secret
	data.Payload = DecodedToken(token, secret.Secret)
	data.SubDocID = secret.SubDocID
	if data.IsValid() {
		c.Set(SignContextKey, &data)
	}
	return &data, nil
}

// EncodedToken
func EncodedToken(claims jwt.MapClaims, secret string) (signed string, err error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err = token.SignedString([]byte(secret))
	if err != nil {
		return
	}
	return
}

// DecodedToken
func DecodedToken(tokenString string, secret string) jwt.MapClaims {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if token != nil {
		claims, ok := token.Claims.(jwt.MapClaims)
		if ok && token.Valid {
			return claims
		}
	}
	ERROR(err)
	return nil
}

// DelAccCache
func DelAccCache() {
	rawDesc, _ := GetValue(PrisDescKey)
	if !IsBlank(rawDesc) {
		desc := rawDesc.(*PrivilegesDesc)
		if desc.IsValid() && desc.SignInfo.Token != "" {
			token := desc.SignInfo.Token
			// 删除redis缓存
			RedisClient.Del(RedisKeyBuilder(RedisSecretKey, token))
			RedisClient.Del(RedisKeyBuilder(RedisOrgKey, token))
		}
	}
}

// Acc
func Acc(handler ...LoginHandlerFunc) *Mod {
	if len(handler) > 0 {
		loginHandler = handler[0]
	}
	return &Mod{
		Code: "acc",
		Models: []interface{}{
			&SignSecret{},
			&SignHistory{},
		},
		Middleware: gin.HandlersChain{
			AuthMiddleware,
		},
		Routes: RoutesInfo{
			LoginRoute,
			LogoutRoute,
			ValidRoute,
			APIKeyRoute,
			WhitelistRoute,
		},
	}
}