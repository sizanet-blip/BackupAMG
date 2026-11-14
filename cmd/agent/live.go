package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"backup-agent/internal/config"
	"backup-agent/internal/logging"
	"backup-agent/internal/panel"
	"backup-agent/internal/paths"
	"backup-agent/internal/report"
	"backup-agent/internal/restic"
	"backup-agent/internal/scheduler"
	"backup-agent/internal/state"
	"backup-agent/internal/status"
	"backup-agent/internal/system"
)

var (
	errBusy     = errors.New("Ya hay una tarea en curso.")
	errNoJob    = errors.New("No hay ninguna tarea en curso.")
	errNotReady = errors.New("El agente todavía no está listo.")
)

// liveJob es la tarea que el panel muestra y puede cancelar.
// El planificador y los botones comparten este estado.
type liveJob struct {
	mu        sync.Mutex
	parent    context.Context
	logger    *logging.Logger
	onPublish func()

	ctx      context.Context
	cancel   context.CancelFunc
	token    *struct{}
	kind     string
	starting bool
	progress restic.Status
	current  string
	pushed   time.Time
	started  time.Time
	trigger  string
	vss      bool

	usedPath  string
	usedBytes int64
	usedKnown bool
	usedAt    time.Time
	restore   *status.RestoreResult
}

var live liveJob

func (l *liveJob) bind(ctx context.Context, logger *logging.Logger, srv *panel.Server) {
	l.mu.Lock()
	l.parent = ctx
	l.logger = logger
	l.onPublish = srv.Publish
	l.mu.Unlock()
	srv.SetHooks(panel.Hooks{
		Status:           l.snapshot,
		Backup:           func(string) error { return l.request(status.ActivityBackup) },
		Verify:           func(string) error { return l.request(status.ActivityVerify) },
		Cancel:           func(string) error { return l.stop() },
		LoadConfig:       loadEditable,
		SaveConfig:       saveEditable,
		LoadDestination:  loadDestination,
		SaveDestination:  saveDestination,
		TestDestination:  testDestination,
		ChangePassword:   changePassword,
		SetupStatus:      setupStatus,
		CompleteSetup:    completeSetup,
		History:          listHistory,
		HistoryItem:      readHistory,
		Snapshots:        listSnapshots,
		SnapshotFiles:    listSnapshotFiles,
		SnapshotFind:     findSnapshot,
		RestoreTarget:    suggestedTarget,
		StartRestore:     startRestore,
		Diagnostics:      diagnostics,
		DiagnosticLog:    diagnosticLog,
		DiagnosticBundle: diagnosticBundle,
		Folders:          listFolders,
		Shortcuts:        folderShortcuts,
		Estimate:         estimateSources,
	})
}

func (l *liveJob) request(kind string) error {
	l.mu.Lock()
	parent := l.parent
	logger := l.logger
	busy := l.cancel != nil || l.starting
	l.mu.Unlock()
	if parent == nil || logger == nil {
		return errNotReady
	}
	if busy {
		return errBusy
	}
	l.mu.Lock()
	if l.cancel != nil || l.starting {
		l.mu.Unlock()
		return errBusy
	}
	l.starting = true
	l.mu.Unlock()

	go func() {
		defer func() {
			l.mu.Lock()
			l.starting = false
			l.mu.Unlock()
			l.publish()
		}()
		if kind == status.ActivityBackup {
			runWithRetries(parent, logger, report.TriggerManual)
			return
		}
		cfg, password, exe, err := openRuntime()
		if err != nil {
			logger.Error("No se pudo preparar la verificación", "error", err.Error())
			return
		}
		if err := executeVerification(parent, logger.Logger, cfg, password, exe, report.TriggerManual); err != nil {
			logger.Error("La verificación no se ha completado", "error", err.Error())
		}
	}()
	return nil
}

func (l *liveJob) stop() error {
	l.mu.Lock()
	cancel := l.cancel
	l.mu.Unlock()
	if cancel == nil {
		return errNoJob
	}
	cancel()
	return nil
}

// enter marca una tarea. Si parent ya es la tarea en curso, solo cambia el rótulo
// (la verificación que sigue a una copia). Otra tarea distinta se rechaza.
func (l *liveJob) enter(parent context.Context, kind string) (context.Context, func(), bool) {
	l.mu.Lock()
	if l.cancel != nil {
		if parent == l.ctx {
			prev := l.kind
			l.kind = kind
			l.mu.Unlock()
			l.publish()
			return parent, func() {
				l.mu.Lock()
				if l.kind == kind {
					l.kind = prev
				}
				l.mu.Unlock()
				l.publish()
			}, true
		}
		l.mu.Unlock()
		return nil, nil, false
	}
	ctx, cancel := context.WithCancel(parent)
	token := &struct{}{}
	l.ctx = ctx
	l.cancel = cancel
	l.token = token
	l.kind = kind
	l.progress = restic.Status{}
	l.current = ""
	l.pushed = time.Time{}
	l.started = time.Now()
	l.trigger = ""
	l.vss = false
	l.mu.Unlock()
	l.publish()
	return ctx, func() { l.finish(token, cancel) }, true
}

func (l *liveJob) finish(token *struct{}, cancel context.CancelFunc) {
	cancel()
	l.mu.Lock()
	if l.token == token {
		l.token = nil
		l.cancel = nil
		l.ctx = nil
		l.kind = ""
		l.progress = restic.Status{}
		l.current = ""
	}
	l.mu.Unlock()
	l.publish()
}

func (l *liveJob) setCue(trigger string, vss bool) {
	l.mu.Lock()
	l.trigger = trigger
	l.vss = vss
	l.mu.Unlock()
	l.publish()
}

func (l *liveJob) note(st restic.Status) {
	l.mu.Lock()
	l.progress = st
	if len(st.CurrentFiles) > 0 {
		l.current = st.CurrentFiles[0]
	}
	send := time.Since(l.pushed) >= 300*time.Millisecond
	if send {
		l.pushed = time.Now()
	}
	l.mu.Unlock()
	if send {
		l.publish()
	}
}

func (l *liveJob) publish() {
	l.mu.Lock()
	fn := l.onPublish
	l.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func (l *liveJob) snapshot() any {
	now := time.Now()
	l.mu.Lock()
	kind := l.kind
	prog := l.progress
	current := l.current
	started := l.started
	trigger := l.trigger
	vss := l.vss
	var restored *status.RestoreResult
	if l.restore != nil {
		copy := *l.restore
		restored = &copy
	}
	l.mu.Unlock()

	view := status.View{Activity: kind}
	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		view.Level = status.Red
		view.Problem = err.Error()
		return view
	}
	view.AgentName = cfg.Agent.Name
	view.Destination.Path = cfg.Repository.Path
	resolved, locErr := system.Locate(cfg.Repository.Path, cfg.Repository.VolumeID)
	if locErr == nil {
		view.Destination.Path = resolved
	}
	info, statErr := os.Stat(view.Destination.Path)
	view.Destination.Connected = locErr == nil && statErr == nil && info.IsDir()
	if view.Destination.Connected {
		if free, total, freeErr := system.DiskSpace(view.Destination.Path); freeErr == nil {
			view.Destination.FreeBytes = free
			view.Destination.TotalBytes = total
		}
		view.Destination.Kind = system.DriveKind(view.Destination.Path)
		view.Destination.Label = system.VolumeLabel(view.Destination.Path)
		used, known := l.repoBytes(view.Destination.Path, kind != "")
		view.Destination.UsedBytes = used
		view.Destination.UsedKnown = known
	}

	st, _ := state.Load(paths.StatePath())
	view.LastBackup = runFromBackup(st.LastBackup)
	view.LastVerify = runFromVerify(st.LastVerify)
	if next, nextErr := scheduler.NextAfterOn(now, cfg.Schedule.Times, cfg.Schedule.Weekdays); nextErr == nil {
		view.NextBackup = next.Format(time.RFC3339)
	}
	_, overdue := scheduler.MostRecentDueOn(now, lastAttemptTime(), cfg.Schedule.Times, cfg.Schedule.Weekdays)
	view.Level = status.Decide(status.Facts{
		Now:        now,
		Activity:   kind,
		Last:       view.LastBackup,
		StaleAfter: time.Duration(cfg.Schedule.StaleAfterDays) * 24 * time.Hour,
		Overdue:    overdue,
	})
	if kind == status.ActivityBackup || kind == status.ActivityRestore {
		speed := 0.0
		if prog.SecondsElapsed > 0 {
			speed = float64(prog.BytesDone) / prog.SecondsElapsed
		}
		percent := prog.PercentDone * 100
		if percent < 0 {
			percent = 0
		}
		if percent > 100 {
			percent = 100
		}
		view.Progress = &status.Progress{
			Percent:          percent,
			FilesDone:        prog.FilesDone,
			FilesTotal:       prog.TotalFiles,
			BytesDone:        prog.BytesDone,
			BytesPerSecond:   speed,
			SecondsRemaining: prog.SecondsRemaining,
			SecondsElapsed:   prog.SecondsElapsed,
			CurrentFile:      current,
		}
		if !started.IsZero() {
			view.Job = &status.Job{
				Trigger:   trigger,
				StartedAt: started.Format(time.RFC3339),
				UseVSS:    vss && kind == status.ActivityBackup,
			}
		}
	}
	view.Restore = restored
	return view
}

func (l *liveJob) repoBytes(path string, busy bool) (int64, bool) {
	l.mu.Lock()
	if l.usedPath == path && l.usedKnown && time.Since(l.usedAt) < time.Minute {
		used, known := l.usedBytes, l.usedKnown
		l.mu.Unlock()
		return used, known
	}
	if busy && l.usedPath == path && l.usedKnown {
		used := l.usedBytes
		l.mu.Unlock()
		return used, true
	}
	l.mu.Unlock()

	total, ok := folderBytes(path)
	l.mu.Lock()
	if ok {
		l.usedPath = path
		l.usedBytes = total
		l.usedKnown = true
		l.usedAt = time.Now()
	}
	l.mu.Unlock()
	return total, ok
}

func folderBytes(root string) (int64, bool) {
	var total int64
	ok := true
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry == nil {
			ok = false
			return nil
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				ok = false
				return nil
			}
			total += info.Size()
		}
		return nil
	})
	return total, err == nil && ok
}

func runFromBackup(last *state.LastBackup) *status.Run {
	if last == nil || last.FinishedAt == "" {
		return nil
	}
	run := &status.Run{FinishedAt: last.FinishedAt, Status: last.Status}
	fillReport(last.Report, run)
	return run
}

func runFromVerify(last *state.LastVerify) *status.Run {
	if last == nil || last.FinishedAt == "" {
		return nil
	}
	run := &status.Run{FinishedAt: last.FinishedAt, Status: last.Status}
	fillReport(last.Report, run)
	return run
}

func fillReport(name string, run *status.Run) {
	if name == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(paths.ReportsDir(), name))
	if err != nil {
		return
	}
	var extra struct {
		DurationSeconds int      `json:"duration_seconds"`
		DataAddedBytes  int64    `json:"data_added_bytes"`
		Message         string   `json:"message"`
		Errors          []string `json:"errors"`
	}
	if json.Unmarshal(data, &extra) != nil {
		return
	}
	run.DurationSeconds = extra.DurationSeconds
	run.DataAddedBytes = extra.DataAddedBytes
	run.Message = restic.PlainFailure(extra.Message)
	raw := strings.TrimSpace(extra.Message)
	if len(extra.Errors) > 0 && strings.TrimSpace(extra.Errors[0]) != "" {
		raw = strings.TrimSpace(extra.Errors[0])
	}
	if raw != "" && raw != run.Message {
		run.Detail = raw
	}
}
