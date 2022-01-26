package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime/debug"
	"time"

	"github.com/caster8013/logv2rayfullstack/database"
	"github.com/caster8013/logv2rayfullstack/model"
	routers "github.com/caster8013/logv2rayfullstack/routers"
	"github.com/caster8013/logv2rayfullstack/routine"
	"github.com/caster8013/logv2rayfullstack/v2ray"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron"
	"github.com/shomali11/parallelizer"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
)

type User = model.User

var cronInstance *cron.Cron

func init() {

	cronInstance = cron.New()
	cronInstance.Start()
}

func main() {

	app := &cli.App{
		Name:  "logv2rayfullstack",
		Usage: "A simple CLI program to manage logv2ray backend",
		Commands: []*cli.Command{
			{
				Name:    "server",
				Aliases: []string{"s"},
				Usage:   "run backend server",
				Action: func(c *cli.Context) error {

					group := parallelizer.NewGroup()
					defer group.Close()

					group.Add(V2rayProcess)
					group.Add(runServer)

					err := group.Wait()

					return err
				},
			},
			{
				Name:    "cron",
				Aliases: []string{"c"},
				Usage:   "run cron job",
				Action: func(c *cli.Context) error {
					return nil
				},
			},
			{
				Name:    "mongo",
				Aliases: []string{"db"},
				Usage:   "manage mongoDB",
				Action: func(c *cli.Context) error {

					tag := c.Args().First()

					switch tag {

					case "tweet":
						err := database.AddDBUserProperty()
						return err

					case "delallusers":
						err := database.EmptyUsersInfoInDB()
						return err

					case "delalltables":
						err := database.DeleteUsersDBs()
						return err

					default:
						fmt.Println(tag)
					}

					return nil
				},
			},
			{
				Name:    "test",
				Aliases: []string{"t"},
				Usage:   "command test",
				Action: func(c *cli.Context) error {
					fmt.Println("added task: ", c.Args().First(), c.Args().Get(2))
					return nil
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

}

func V2rayProcess() {

	V2RAY := os.Getenv("V2RAY")
	V2RAY_CONFIG := os.Getenv("V2RAY_CONFIG")

	var cmd = exec.Command(V2RAY, "-config", V2RAY_CONFIG)
	if err := cmd.Run(); err != nil {
		log.Panic("Panic: ", err)
	}
}

func runServer() {
	// wait v2ray process to be ready.
	time.Sleep(time.Second)

	var V2_API_ADDRESS = os.Getenv("V2_API_ADDRESS")
	var V2_API_PORT = os.Getenv("V2_API_PORT")

	var SERVER_ADDRESS = os.Getenv("SERVER_ADDRESS")
	var SERVER_PORT = os.Getenv("SERVER_PORT")

	allUsersInDB, _ := database.GetAllUsersInfo()
	if len(allUsersInDB) != 0 {

		cmdConn, err := grpc.Dial(fmt.Sprintf("%s:%s", V2_API_ADDRESS, V2_API_PORT), grpc.WithInsecure())
		if err != nil {
			log.Panic(err)
		}

		for _, user := range allUsersInDB {
			if user.Status == "plain" {
				NHSClient := v2ray.NewHandlerServiceClient(cmdConn, user.Path)
				NHSClient.AddUser(*user)
			}
		}
	}
	// add cron
	routine.Cron_loggingJobs(cronInstance)

	// default:
	// router := gin.Default()

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(recoverFromError)

	routers.AuthRoutes(router)
	routers.UserRoutes(router)

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"final error": "no route found."})
	})

	router.Run(fmt.Sprintf("%s:%s", SERVER_ADDRESS, SERVER_PORT))

}

func recoverFromError(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {

			// 打印错误堆栈信息
			log.Panicf("Panic: %v\n", r)
			debug.PrintStack()

			// 用json封装信息返回
			c.JSON(200, gin.H{"code": 4444, "message": "Server internal error!"})
		}
	}()

	// 加载完defer recover, 继续后续接口调用
	c.Next()
}
