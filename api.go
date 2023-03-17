package cron

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/yinyajun/cron-admin"
)

func renderJson(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	resp := map[string]interface{}{
		"code": 0,
		"data": data,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderErrJson(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	resp := map[string]interface{}{
		"code": code,
		"msg":  msg,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func newAddHandlerFunc(agent *Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		spec := query.Get("spec")
		job := query.Get("job")
		if err := agent.Add(spec, job); err != nil {
			renderErrJson(w, 1000, err.Error())
			return
		}
		renderJson(w, "ok")
	}
}

func newActiveHandlerFunc(agent *Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		job := query.Get("job")
		if err := agent.Active(job); err != nil {
			renderErrJson(w, 1001, err.Error())
			return
		}
		fmt.Println(">>", "active", job)
		renderJson(w, "ok")
	}
}

func newPauseHandlerFunc(agent *Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		job := query.Get("job")
		if err := agent.Pause(job); err != nil {
			renderErrJson(w, 1002, err.Error())
			return
		}
		fmt.Println(">>", "pause", job)
		renderJson(w, "ok")
	}
}

func newRemoveHandlerFunc(agent *Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		job := query.Get("job")
		if err := agent.Remove(job); err != nil {
			renderErrJson(w, 1003, err.Error())
			return
		}
		renderJson(w, "ok")
	}
}

func newExecuteOnceHandlerFunc(agent *Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		job := query.Get("job")
		if err := agent.ExecuteOnce(job); err != nil {
			renderErrJson(w, 1004, err.Error())
			return
		}
		renderJson(w, "ok")
	}
}

func newScheduleHandlerFunc(agent *Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		events, err := agent.Schedule()
		if err != nil {
			renderErrJson(w, 1005, err.Error())
			return
		}
		renderJson(w, events)
	}
}

func newRunningHandlerFunc(agent *Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		executions, err := agent.Running()
		if err != nil {
			renderErrJson(w, 1006, err.Error())
			return
		}
		renderJson(w, executions)
	}
}

func newHistoryHandlerFunc(agent *Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		job := query.Get("job")
		executions, err := agent.History(job)
		if err != nil {
			renderErrJson(w, 1007, err.Error())
			return
		}
		renderJson(w, executions)
	}
}

func newJobsHandlerFunc(agent *Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderJson(w, agent.Jobs())
	}
}

type GroupRouter struct {
	prefix string
	mux    *http.ServeMux
}

func (g GroupRouter) RegisterHandler(pattern string, f http.HandlerFunc) {
	g.mux.Handle(g.prefix+pattern, f)
}

func Router(agent *Agent) http.Handler {
	mux := http.NewServeMux()

	r := GroupRouter{prefix: "/api/v1", mux: mux}
	r.RegisterHandler("/add", newAddHandlerFunc(agent))
	r.RegisterHandler("/active", newActiveHandlerFunc(agent))
	r.RegisterHandler("/pause", newPauseHandlerFunc(agent))
	r.RegisterHandler("/remove", newRemoveHandlerFunc(agent))
	r.RegisterHandler("/execute", newExecuteOnceHandlerFunc(agent))
	r.RegisterHandler("/running", newRunningHandlerFunc(agent))
	r.RegisterHandler("/schedule", newScheduleHandlerFunc(agent))
	r.RegisterHandler("/history", newHistoryHandlerFunc(agent))
	r.RegisterHandler("/jobs", newJobsHandlerFunc(agent))

	mux.Handle("/", admin.UIHandler())

	return mux
}
