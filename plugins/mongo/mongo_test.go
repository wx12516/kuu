package mongo

import (
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kuuland/kuu"
	"github.com/kuuland/kuu/plugins/mongo"
)

var uri = "mongodb://root:kuuland@127.0.0.1:27017/kuu?authSource=admin&maxPoolSize=50"

func TestGetUseDB(t *testing.T) {
	fmt.Println(parseDB(uri))
}

func TestConnect(t *testing.T) {
	Connect(uri)
	if n, err := C("user").Count(); err == nil {
		fmt.Println(n)
	} else {
		fmt.Println(err)
	}
}

func TestImport(t *testing.T) {
	kuu.Import(Mongo())

	k := kuu.New(kuu.H{
		"mongo": "mongodb://root:kuuland@127.0.0.1:27017/kuu?authSource=admin&maxPoolSize=50",
	})
	k.GET("/", func(c *gin.Context) {
		user := mongo.C("user")
		n, err := user.Count()
		if err == nil {
			c.String(200, string(n))
		} else {
			c.String(200, err.Error())
		}
	})

	k.Run(":8080")
}
