package web

import (
	"asyncProxy/web/views"
	"asyncProxy/ws"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	recover2 "github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"
	"log"
	"net/http"
)

func Start(host string, port uint16, username, password string) {
	engine := html.NewFileSystem(http.FS(views.Views), ".html")
	engine.Reload(false)
	engine.Debug(false)
	engine.Delims("[[", "]]")
	set := ws.EdgeSet
	app := fiber.New(fiber.Config{
		Views:                 engine,
		DisableStartupMessage: true,
		ErrorHandler: func(ctx *fiber.Ctx, err error) error {
			log.Println("web error:", err)
			return ctx.SendString("error")
		},
	})
	app.Use(recover2.New(recover2.Config{
		EnableStackTrace: true,
	}))
	auth := basicauth.New(basicauth.Config{
		Users: map[string]string{
			username: password,
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
	log.Println("web is started")
	err := app.Listen(fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		log.Fatalln("error while starting http server:", err)
		return
	}
}
