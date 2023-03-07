# cron
simple distributed cron


## usage

```golang
var bindPort = flag.Int("port", 0, "")

func main() {
	flag.Parse()

	cli := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	store := cron.NewRedisKV(cli, "_cron")

	timeline := cron.NewRedisTimeline("_cron", cli)
	executor := cron.NewChanExecutor(map[string]cron.Job{
		"t1": task{"t1"},
		"t2": task{"t2"},
		"t3": task{"t3"},
	})

	cfg := memberlist.DefaultLANConfig()
	cfg.Name = fmt.Sprintf("test_%d", *bindPort)
	cfg.AdvertisePort = *bindPort
	cfg.BindPort = *bindPort

	entries := cron.NewGossipEntries(store, cfg,
		[]string{"127.0.0.1:8001", "127.0.0.1:8002"})

	c := cron.New(timeline, executor, entries)

	c.Add("@every 5s", "t1")
	c.Activate("t1")

	select {}
}
```

`go run main.go --port=8001`
```shell
2023/03/07 21:54:35 [DEBUG] memberlist: Stream connection from=127.0.0.1:56681
2023/03/07 21:54:35 [DEBUG] memberlist: Initiating push/pull sync with:  127.0.0.1:8001
2023/03/07 21:54:35 [DEBUG] memberlist: Failed to join 127.0.0.1:8002: dial tcp 127.0.0.1:8002: connect: connection refused
DEBU[2023-03-07T21:54:35+08:00] restore 0 events from timeline
DEBU[2023-03-07T21:54:35+08:00] add: {"name":"t1","spec":"@every 5s"}
DEBU[2023-03-07T21:54:35+08:00] add: {"name":"t2","spec":"@every 3s"}
INFO[2023-03-07T21:54:35+08:00] dispense: {"name":"t1","spec":"@every 5s"}
run t1
INFO[2023-03-07T21:54:35+08:00] dispense: {"name":"t2","spec":"@every 3s"}
run t2
INFO[2023-03-07T21:54:38+08:00] dispense: {"name":"t2","spec":"@every 3s"}
run t2
INFO[2023-03-07T21:54:40+08:00] dispense: {"name":"t1","spec":"@every 5s"}
run t1
2023/03/07 21:54:40 [DEBUG] memberlist: Stream connection from=127.0.0.1:56701
INFO[2023-03-07T21:54:43+08:00] dispense: {"name":"t2","spec":"@every 3s"}
run t2
INFO[2023-03-07T21:54:45+08:00] dispense: {"name":"t1","spec":"@every 5s"}
run t1
INFO[2023-03-07T21:54:46+08:00] dispense: {"name":"t2","spec":"@every 3s"}
run t2
INFO[2023-03-07T21:54:49+08:00] dispense: {"name":"t2","spec":"@every 3s"}
run t2
INFO[2023-03-07T21:54:52+08:00] dispense: {"name":"t2","spec":"@every 3s"}
run t2
INFO[2023-03-07T21:54:55+08:00] dispense: {"name":"t1","spec":"@every 5s"}
run t1
INFO[2023-03-07T21:54:55+08:00] dispense: {"name":"t2","spec":"@every 3s"}
run t2
INFO[2023-03-07T21:54:58+08:00] dispense: {"name":"t2","spec":"@every 3s"}
run t2
INFO[2023-03-07T21:55:00+08:00] dispense: {"name":"t1","spec":"@every 5s"}
run t1
INFO[2023-03-07T21:55:01+08:00] dispense: {"name":"t2","spec":"@every 3s"}
run t2
INFO[2023-03-07T21:55:04+08:00] dispense: {"name":"t2","spec":"@every 3s"}
run t2
INFO[2023-03-07T21:55:07+08:00] dispense: {"name":"t2","spec":"@every 3s"}
run t2
INFO[2023-03-07T21:55:13+08:00] dispense: {"name":"t2","spec":"@every 3s"}
run t2

```

`go run main.go --port=8002`
```shell
2023/03/07 21:54:40 [DEBUG] memberlist: Initiating push/pull sync with:  127.0.0.1:8001
2023/03/07 21:54:40 [DEBUG] memberlist: Initiating push/pull sync with:  127.0.0.1:8002
2023/03/07 21:54:40 [DEBUG] memberlist: Stream connection from=127.0.0.1:56702
DEBU[2023-03-07T21:54:40+08:00] restore 2 events from timeline
DEBU[2023-03-07T21:54:40+08:00] add: {"name":"t1","spec":"@every 5s"}
DEBU[2023-03-07T21:54:40+08:00] add: {"name":"t2","spec":"@every 3s"}
INFO[2023-03-07T21:54:40+08:00] dispense: {"name":"t1","spec":"@every 5s"}
run t1
INFO[2023-03-07T21:54:40+08:00] dispense: {"name":"t2","spec":"@every 3s"}
run t2
run t1
INFO[2023-03-07T21:54:50+08:00] dispense: {"name":"t1","spec":"@every 5s"}
run t1
INFO[2023-03-07T21:55:05+08:00] dispense: {"name":"t1","spec":"@every 5s"}
INFO[2023-03-07T21:55:10+08:00] dispense: {"name":"t1","spec":"@every 5s"}
run t1
INFO[2023-03-07T21:55:10+08:00] dispense: {"name":"t2","spec":"@every 3s"}
run t2
```
