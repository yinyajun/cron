package cron

import (
	"encoding/json"
	"net/http"

	"github.com/yinyajun/cron-admin"
)

const (
	ErrCodeAdd      = 1000
	ErrCodeActive   = 1001
	ErrCodePause    = 1002
	ErrCodeRemove   = 1003
	ErrCodeExecute  = 1004
	ErrCodeSchedule = 1005
	ErrCodeRunning  = 1006
	ErrCodeHistory  = 1007
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
			renderErrJson(w, ErrCodeAdd, err.Error())
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
			renderErrJson(w, ErrCodeActive, err.Error())
			return
		}
		renderJson(w, "ok")
	}
}

func newPauseHandlerFunc(agent *Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		job := query.Get("job")
		if err := agent.Pause(job); err != nil {
			renderErrJson(w, ErrCodePause, err.Error())
			return
		}
		renderJson(w, "ok")
	}
}

func newRemoveHandlerFunc(agent *Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		job := query.Get("job")
		if err := agent.Remove(job); err != nil {
			renderErrJson(w, ErrCodeRemove, err.Error())
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
			renderErrJson(w, ErrCodeExecute, err.Error())
			return
		}
		renderJson(w, "ok")
	}
}

func newScheduleHandlerFunc(agent *Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		events, err := agent.Schedule()
		if err != nil {
			renderErrJson(w, ErrCodeSchedule, err.Error())
			return
		}
		renderJson(w, events)
	}
}

func newRunningHandlerFunc(agent *Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		executions, err := agent.Running()
		if err != nil {
			renderErrJson(w, ErrCodeRunning, err.Error())
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
			renderErrJson(w, ErrCodeHistory, err.Error())
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

func newMembersHandlerFunc(agent *Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderJson(w, agent.Members())
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
	r.RegisterHandler("/members", newMembersHandlerFunc(agent))

	mux.Handle("/", admin.UIHandler())

	return mux
}
