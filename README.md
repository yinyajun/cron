# cron
cron is a simple distributed cron service, you can import it as as library in your application.

## Framework
<img src="./doc/cron.png" alt="diagram" style="zoom: 33%;" />

## Usage

```golang
func main() {
	conf := &cron.Conf{}
	cron.ReadConfig(conf, *file)

	agent := cron.NewAgent(conf)
	agent.Join(strings.Split(*nodes, ","))

	if err := agent.Register(jobs...); err != nil {
		cron.Logger.Fatalln(err)
	}

	agent.Run()
}
```



## Run Example

```
go run examples/main.go
```


```
WARN[2023-03-18T23:47:12+08:00] ReadConfig failed: open ./conf.json: no such file or directory
2023/03/18 23:47:12 [DEBUG] memberlist: Initiating push/pull sync with:  127.0.0.1:7946
2023/03/18 23:47:12 [DEBUG] memberlist: Stream connection from=127.0.0.1:58651
INFO[2023-03-18T23:47:12+08:00] start admin http server: :8080
```



## Config File

```json
{
  "base": {
    "redis": {
      "addr": "",
      "password": ""
    }
    "http_addr": ""
  },
  "custom": {
    "key_entry": "",
    "key_executor": "",
    "key_timeline": "",
    "max_history_num": 0
  },
  "gossip": {
    "bind_addr": "",
    "bind_port": 0,
    "network": "",
    "node_name": ""
  }
}
```
Generate `config.json` file  and use following code to parse config file.

```go
conf := &cron.Conf{}
cron.ReadConfig(conf, "./config.json")
```

Agent comes with a set of default configuration.  

| Key               | Default   | Explaination                                   |
| :---------------- | --------- | ------------------------------------------------ |
| base.redis        | nil       | redis.Options will init address "127.0.0.1:6379" |
| base.http_addr    | :8080     | agent http server port |
| gossip.network | LAN       | gossip network type                              |
| gossip.bind_addr | 0.0.0.0       | gossip bind addr                              |
| gossip.bind_port | 7946       | gossip bind port                              |
| gossip.node_name  | $hostname |  gossip node name|
| custom.key_timeline | _timeline | custom timeline key in redis                     |
| custom.key_entry  | _entry    | custom entry key in redis                        |
| custom.key_executor | _exe      | custom executor key in redis                     |
| custom.max_history_num | 5         | maximum  number of job history                   |

## API

| Url                | Explaination                          |
| ------------------ | ------------------------------------- |
| `/api/v1/add`      | Add a job schedule                    |
| `/api/v1/active`   | Active the job schedule               |
| `/api/v1/pause`    | Pause the job schedule                |
| `/api/v1/remove`   | Remove the job schedule               |
| `/api/v1/execute`  | Execute the job immediately           |
| `/api/v1/running`  | Fetch the running execution           |
| `/api/v1/schedule` | Fetch all schedule                    |
| `/api/v1/history`  | Fetch the history executions of a job |
| `/api/v1/jobs`     | Fetch all supported jobs              |
| `/api/v1/members`  | Fetch cron members                    |



## WebUI

![ui](doc/ui.png)

