package cron

import (
	"encoding/json"
	"net/http"
	"strconv"

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
		offset, _ := strconv.ParseInt(query.Get("offset"), 10, 64)
		size, _ := strconv.ParseInt(query.Get("size"), 10, 64)
		executions, total, err := agent.History(job, offset, size)
		if err != nil {
			renderErrJson(w, ErrCodeHistory, err.Error())
			return
		}
		renderJson(w, map[string]interface{}{
			"executions": executions,
			"total":      total,
		})
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

func (a *Agent) Router() http.Handler {
	mux := http.NewServeMux()

	r := GroupRouter{prefix: "/api/v1", mux: mux}
	r.RegisterHandler("/add", newAddHandlerFunc(a))
	r.RegisterHandler("/active", newActiveHandlerFunc(a))
	r.RegisterHandler("/pause", newPauseHandlerFunc(a))
	r.RegisterHandler("/remove", newRemoveHandlerFunc(a))
	r.RegisterHandler("/execute", newExecuteOnceHandlerFunc(a))
	r.RegisterHandler("/running", newRunningHandlerFunc(a))
	r.RegisterHandler("/schedule", newScheduleHandlerFunc(a))
	r.RegisterHandler("/history", newHistoryHandlerFunc(a))
	r.RegisterHandler("/jobs", newJobsHandlerFunc(a))
	r.RegisterHandler("/members", newMembersHandlerFunc(a))

	mux.Handle("/", admin.UIHandler())

	return mux
}
