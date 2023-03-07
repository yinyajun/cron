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
2023/03/07 21:42:02 [DEBUG] memberlist: Initiating push/pull sync with:  127.0.0.1:8001
2023/03/07 21:42:02 [DEBUG] memberlist: Stream connection from=127.0.0.1:56262
2023/03/07 21:42:02 [DEBUG] memberlist: Failed to join 127.0.0.1:8002: dial tcp 127.0.0.1:8002: connect: connection refused
run t1
INFO[0000] dispense: {"name":"t1","spec":"@every 5s"}
INFO[0000] dispense: {"name":"t2","spec":"@every 3s"}
run t2
run t2
INFO[0002] dispense: {"name":"t2","spec":"@every 3s"}
INFO[0004] dispense: {"name":"t1","spec":"@every 5s"}
run t1
2023/03/07 21:42:07 [DEBUG] memberlist: Stream connection from=127.0.0.1:56271
INFO[0013] dispense: {"name":"t2","spec":"@every 3s"}
run t2
INFO[0016] dispense: {"name":"t2","spec":"@every 3s"}
run t2
INFO[0019] dispense: {"name":"t2","spec":"@every 3s"}
run t2
```

`go run main.go --port=8002`
```shell
2023/03/07 21:42:07 [DEBUG] memberlist: Initiating push/pull sync with:  127.0.0.1:8001
2023/03/07 21:42:07 [DEBUG] memberlist: Initiating push/pull sync with:  127.0.0.1:8002
2023/03/07 21:42:07 [DEBUG] memberlist: Stream connection from=127.0.0.1:56272
INFO[0000] dispense: {"name":"t1","spec":"@every 5s"}
run t1
INFO[0000] dispense: {"name":"t2","spec":"@every 3s"}
run t2
INFO[0002] dispense: {"name":"t2","spec":"@every 3s"}
run t2
INFO[0004] dispense: {"name":"t1","spec":"@every 5s"}
run t1
INFO[0005] dispense: {"name":"t2","spec":"@every 3s"}
run t2
INFO[0009] dispense: {"name":"t1","spec":"@every 5s"}
run t1
INFO[0014] dispense: {"name":"t1","spec":"@every 5s"}
run t1

```
