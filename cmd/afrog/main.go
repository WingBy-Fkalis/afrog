package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
	"github.com/zan8in/afrog/internal/runner"
	"github.com/zan8in/afrog/pkg/config"
	"github.com/zan8in/afrog/pkg/core"
	"github.com/zan8in/afrog/pkg/html"
	"github.com/zan8in/afrog/pkg/log"
	"github.com/zan8in/afrog/pkg/poc"
	"github.com/zan8in/afrog/pkg/upgrade"
)

var options = &config.Options{}
var htemplate = &html.HtmlTemplate{}

func main() {
	app := cli.NewApp()
	app.Name = runner.ShowBanner(config.Version)
	app.Usage = " "
	app.UsageText = "afrog [命令]"
	app.Version = config.Version

	app.Flags = []cli.Flag{
		&cli.StringFlag{Name: "target", Aliases: []string{"t"}, Destination: &options.Target, Value: "", Usage: "指定扫描的URL/Host"},
		&cli.StringFlag{Name: "targetFilePath", Aliases: []string{"T"}, Destination: &options.TargetsFilePath, Value: "", Usage: "指定需要扫描的URL/Host文件（一行一个）"},
		&cli.StringFlag{Name: "PocsFilePath", Aliases: []string{"P"}, Destination: &options.PocsFilePath, Value: "", Usage: "指定需要扫描的POC脚本的路径（非必须，默认加载{home}/afrog-pocs）"},
		&cli.StringFlag{Name: "Output", Aliases: []string{"o"}, Destination: &options.Output, Value: "", Usage: "输出扫描结果到html文件，比如：-o result.html"},
	}

	app.Action = func(c *cli.Context) error {

		title := log.LogColor.Vulner("A tool for finding vulnerabilities - afrog V" + config.Version)

		upgrade := upgrade.New()
		upgrade.UpgradeAfrogPocs()

		defconfig := log.LogColor.Low("Default Conf  " + options.Config.GetConfigPath())
		defpocdir := log.LogColor.Low("Default Pocs  " + poc.GetPocPath())

		fmt.Println(title + "\r\n" + defconfig + "\r\n" + defpocdir + " v" + upgrade.LastestVersion + "")

		htemplate.Filename = options.Output
		if err := htemplate.New(); err != nil {
			return err
		}

		err := runner.New(options, func(result interface{}) {
			r := result.(*core.Result)

			options.OptLock.Lock()
			defer options.OptLock.Unlock()

			options.CurrentCount++

			if r.IsVul {
				r.PrintColorResultInfoConsole()

				if len(r.Output) > 0 {
					htemplate.Result = r
					htemplate.Append()
				}
			}

			fmt.Printf("\r%d/%d | %d%% ", options.CurrentCount, options.Count, options.CurrentCount*100/options.Count)
		})
		if err != nil {
			return err
		}

		return err
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(log.LogColor.High("Failed to start afrog，", err.Error()))
	}
}

func PrintTraceInfo(result *core.Result) {
	for i, v := range result.AllPocResult {
		log.Log().Info(fmt.Sprintf("\r\n%s（%d）\r\n%s\r\n\r\n%s（%d）\r\n%s\r\n", "Request:", i, v.ReadFullResultRequestInfo(), "Response:", i, v.ReadFullResultResponseInfo()))
	}
}
