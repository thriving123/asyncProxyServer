package web

import (
	"asyncProxy/ws"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/gofiber/template/html/v2"
	"log"
)

func Start(host string, port uint16) {
	engine := html.New("./web/views", ".html")
	engine.Reload(true)
	engine.Debug(false)
	engine.Delims("[[", "]]")
	set := ws.EdgeSet
	app := fiber.New(fiber.Config{
		Views: engine,
	})
	auth := basicauth.New(basicauth.Config{
		Users: map[string]string{
			"admin": "admin",
		},
	})
	app.Get("/", auth, func(c *fiber.Ctx) error {
		data := set.Data()
		list := make([]string, 0)
		for _, edge := range data {
			addr := edge.Conn.RemoteAddr()
			if addr != nil {
				list = append(list, addr.String())
			}
		}
		return c.Render("index", fiber.Map{
			"Total": set.Len(),
			"List":  list,
		})
	})
	app.Listen(fmt.Sprintf("%s:%d", host, port))
	log.Println("web is started")
}
