package config

import (
	"bytes"
	"os"

	"gopkg.in/yaml.v3"
)

// Editable es lo que el panel puede cambiar. El destino y la contraseña se conservan.
type Editable struct {
	Sources            []string `json:"sources"`
	Excludes           []string `json:"excludes"`
	Times              []string `json:"times"`
	Weekdays           []int    `json:"weekdays"`
	RunMissedOnStartup bool     `json:"run_missed_on_startup"`
	MaxRetries         int      `json:"max_retries"`
	RetryDelayMinutes  int      `json:"retry_delay_minutes"`
	StaleAfterDays     int      `json:"stale_after_days"`
	KeepDaily          int      `json:"keep_daily"`
	KeepWeekly         int      `json:"keep_weekly"`
	KeepMonthly        int      `json:"keep_monthly"`
	KeepYearly         int      `json:"keep_yearly"`
	Prune              bool     `json:"prune"`
	EveryDays          int      `json:"every_days"`
	ReadDataSubset     string   `json:"read_data_subset"`
}

// Update cambia las carpetas, el horario, la retención y la verificación.
// Si el resultado no es válido, el fichero no se toca.
func Update(path string, edit Editable) error {
	current, err := Load(path)
	if err != nil {
		return err
	}
	next := *current
	next.Backup.Sources = edit.Sources
	next.Backup.Excludes = edit.Excludes
	next.Schedule.Times = edit.Times
	next.Schedule.Weekdays = normalizeWeekdays(edit.Weekdays)
	next.Schedule.RunMissedOnStartup = edit.RunMissedOnStartup
	next.Schedule.MaxRetries = edit.MaxRetries
	next.Schedule.RetryDelayMinutes = edit.RetryDelayMinutes
	next.Schedule.StaleAfterDays = edit.StaleAfterDays
	next.Retention.KeepDaily = edit.KeepDaily
	next.Retention.KeepWeekly = edit.KeepWeekly
	next.Retention.KeepMonthly = edit.KeepMonthly
	next.Retention.KeepYearly = edit.KeepYearly
	next.Retention.Prune = edit.Prune
	next.Verify.EveryDays = edit.EveryDays
	next.Verify.ReadDataSubset = edit.ReadDataSubset

	raw, err := marshalFile(next)
	if err != nil {
		return err
	}
	if _, err := Parse(raw); err != nil {
		return err
	}
	return atomicWrite(path, raw)
}

// View devuelve los datos editables, sin la contraseña ni el destino.
func (cfg Config) View() Editable {
	days := cfg.Schedule.Weekdays
	if len(days) == 0 {
		days = []int{1, 2, 3, 4, 5, 6, 7}
	}
	excludes := cfg.Backup.Excludes
	if excludes == nil {
		excludes = []string{}
	}
	return Editable{
		Sources:            cfg.Backup.Sources,
		Excludes:           excludes,
		Times:              cfg.Schedule.Times,
		Weekdays:           days,
		RunMissedOnStartup: cfg.Schedule.RunMissedOnStartup,
		MaxRetries:         cfg.Schedule.MaxRetries,
		RetryDelayMinutes:  cfg.Schedule.RetryDelayMinutes,
		StaleAfterDays:     cfg.Schedule.StaleAfterDays,
		KeepDaily:          cfg.Retention.KeepDaily,
		KeepWeekly:         cfg.Retention.KeepWeekly,
		KeepMonthly:        cfg.Retention.KeepMonthly,
		KeepYearly:         cfg.Retention.KeepYearly,
		Prune:              cfg.Retention.Prune,
		EveryDays:          cfg.Verify.EveryDays,
		ReadDataSubset:     cfg.Verify.ReadDataSubset,
	}
}

func normalizeWeekdays(days []int) []int {
	if len(days) == 7 {
		seen := map[int]struct{}{}
		for _, day := range days {
			seen[day] = struct{}{}
		}
		if len(seen) == 7 {
			return nil
		}
	}
	return days
}

func marshalFile(cfg Config) ([]byte, error) {
	type agent struct {
		Name string `yaml:"name"`
	}
	type backup struct {
		Sources  []string `yaml:"sources"`
		Excludes []string `yaml:"excludes,omitempty"`
		UseVSS   bool     `yaml:"use_vss"`
	}
	type repository struct {
		Path     string `yaml:"path"`
		VolumeID string `yaml:"volume_id,omitempty"`
		Password string `yaml:"password_encrypted"`
	}
	type schedule struct {
		Times              []string `yaml:"times"`
		Weekdays           []int    `yaml:"weekdays,omitempty"`
		RunMissedOnStartup bool     `yaml:"run_missed_on_startup"`
		MaxRetries         int      `yaml:"max_retries"`
		RetryDelayMinutes  int      `yaml:"retry_delay_minutes"`
		StaleAfterDays     int      `yaml:"stale_after_days"`
	}
	type retention struct {
		KeepDaily   int  `yaml:"keep_daily"`
		KeepWeekly  int  `yaml:"keep_weekly"`
		KeepMonthly int  `yaml:"keep_monthly"`
		KeepYearly  int  `yaml:"keep_yearly"`
		Prune       bool `yaml:"prune"`
	}
	type verify struct {
		EveryDays      int    `yaml:"every_days"`
		ReadDataSubset string `yaml:"read_data_subset"`
	}
	type panel struct {
		Port int `yaml:"port"`
	}
	doc := struct {
		Agent      agent         `yaml:"agent"`
		Backup     backup        `yaml:"backup"`
		Repository repository    `yaml:"repository"`
		Schedule   schedule      `yaml:"schedule"`
		Retention  retention     `yaml:"retention"`
		Verify     verify        `yaml:"verify"`
		Panel      panel         `yaml:"panel"`
		Replicas   []fileReplica `yaml:"replicas,omitempty"`
	}{
		Agent: agent{Name: cfg.Agent.Name},
		Backup: backup{
			Sources:  cfg.Backup.Sources,
			Excludes: cfg.Backup.Excludes,
			UseVSS:   cfg.Backup.UseVSS,
		},
		Repository: repository{Path: cfg.Repository.Path, VolumeID: cfg.Repository.VolumeID, Password: cfg.Repository.PasswordEncrypted},
		Schedule: schedule{
			Times:              cfg.Schedule.Times,
			Weekdays:           cfg.Schedule.Weekdays,
			RunMissedOnStartup: cfg.Schedule.RunMissedOnStartup,
			MaxRetries:         cfg.Schedule.MaxRetries,
			RetryDelayMinutes:  cfg.Schedule.RetryDelayMinutes,
			StaleAfterDays:     cfg.Schedule.StaleAfterDays,
		},
		Retention: retention{
			KeepDaily:   cfg.Retention.KeepDaily,
			KeepWeekly:  cfg.Retention.KeepWeekly,
			KeepMonthly: cfg.Retention.KeepMonthly,
			KeepYearly:  cfg.Retention.KeepYearly,
			Prune:       cfg.Retention.Prune,
		},
		Verify: verify{EveryDays: cfg.Verify.EveryDays, ReadDataSubset: cfg.Verify.ReadDataSubset},
		Panel:  panel{Port: cfg.Panel.Port},
		// Las copias adicionales se conservan al guardar Ajustes.
		Replicas: fileReplicas(cfg.Replicas),
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			_ = os.Remove(tmp)
			return err2
		}
	}
	return nil
}
