package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"backup-agent/internal/secrets"
)

// volumeIDPattern es el nombre de volumen de Windows, por ejemplo \\?\Volume{guid}\.
var volumeIDPattern = regexp.MustCompile(`(?i)^\\\\\?\\Volume\{[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}\}\\$`)

const (
	defaultMaxRetries        = 2
	defaultRetryDelayMinutes = 15
	defaultKeepDaily         = 7
	defaultKeepWeekly        = 4
	defaultKeepMonthly       = 12
	defaultEveryDays         = 7
	defaultReadDataSubset    = "5%"
	defaultStaleAfterDays    = 2
)

type rawFile struct {
	Agent      *rawAgent      `yaml:"agent"`
	Backup     *rawBackup     `yaml:"backup"`
	Repository *rawRepository `yaml:"repository"`
	Schedule   *rawSchedule   `yaml:"schedule"`
	Retention  *rawRetention  `yaml:"retention"`
	Verify     *rawVerify     `yaml:"verify"`
	Panel      *rawPanel      `yaml:"panel"`
	Replicas   []rawReplica   `yaml:"replicas"`
	Cloud      *rawCloud      `yaml:"cloud"`
}

type rawAgent struct {
	Name *string `yaml:"name"`
}

type rawBackup struct {
	Sources  []string `yaml:"sources"`
	Excludes []string `yaml:"excludes"`
	UseVSS   *bool    `yaml:"use_vss"`
}

type rawRepository struct {
	Path              *string `yaml:"path"`
	VolumeID          *string `yaml:"volume_id"`
	PasswordEncrypted *string `yaml:"password_encrypted"`
}

type rawSchedule struct {
	Times              []string `yaml:"times"`
	RunMissedOnStartup *bool    `yaml:"run_missed_on_startup"`
	MaxRetries         *int     `yaml:"max_retries"`
	RetryDelayMinutes  *int     `yaml:"retry_delay_minutes"`
	StaleAfterDays     *int     `yaml:"stale_after_days"`
	Weekdays           []int    `yaml:"weekdays"`
}

type rawRetention struct {
	KeepDaily   *int  `yaml:"keep_daily"`
	KeepWeekly  *int  `yaml:"keep_weekly"`
	KeepMonthly *int  `yaml:"keep_monthly"`
	KeepYearly  *int  `yaml:"keep_yearly"`
	Prune       *bool `yaml:"prune"`
}

type rawVerify struct {
	EveryDays      *int    `yaml:"every_days"`
	ReadDataSubset *string `yaml:"read_data_subset"`
}

type rawPanel struct {
	Port *int `yaml:"port"`
}

var hostname = os.Hostname

func (raw rawFile) validate() (*Config, error) {
	var problems []string
	cfg := &Config{
		Backup:    Backup{UseVSS: true},
		Schedule:  Schedule{RunMissedOnStartup: true, MaxRetries: defaultMaxRetries, RetryDelayMinutes: defaultRetryDelayMinutes, StaleAfterDays: defaultStaleAfterDays},
		Retention: Retention{KeepDaily: defaultKeepDaily, KeepWeekly: defaultKeepWeekly, KeepMonthly: defaultKeepMonthly, Prune: true},
		Verify:    Verify{EveryDays: defaultEveryDays, ReadDataSubset: defaultReadDataSubset},
		Panel:     Panel{Port: DefaultPanelPort},
	}

	problems = append(problems, applyAgent(raw.Agent, cfg)...)
	if raw.Backup != nil {
		problems = append(problems, applyBackup(*raw.Backup, cfg)...)
	} else {
		problems = append(problems, "backup.sources: indica al menos una carpeta de origen")
	}
	if raw.Repository != nil {
		problems = append(problems, applyRepository(*raw.Repository, cfg)...)
	} else {
		problems = append(problems, "repository.path: indica la carpeta de destino")
		problems = append(problems, "repository.password_encrypted: la contraseña no está configurada. Se hará con el comando agent set-password")
	}
	if raw.Schedule != nil {
		problems = append(problems, applySchedule(*raw.Schedule, cfg)...)
	} else {
		problems = append(problems, "schedule.times: indica al menos una hora (formato HH:MM)")
	}
	if raw.Retention != nil {
		problems = append(problems, applyRetention(*raw.Retention, cfg)...)
	}
	if raw.Verify != nil {
		problems = append(problems, applyVerify(*raw.Verify, cfg)...)
	}
	if raw.Panel != nil {
		problems = append(problems, applyPanel(*raw.Panel, cfg)...)
	}
	problems = append(problems, applyReplicas(raw.Replicas, cfg)...)
	problems = append(problems, applyCloud(raw.Cloud, cfg)...)

	if len(problems) > 0 {
		return nil, &ValidationError{Problems: problems}
	}
	return cfg, nil
}

func applyAgent(raw *rawAgent, cfg *Config) []string {
	name := ""
	if raw != nil && raw.Name != nil {
		name = strings.TrimSpace(*raw.Name)
	}
	if name == "" {
		host, err := hostname()
		if err != nil || strings.TrimSpace(host) == "" {
			return []string{"agent.name: no se pudo obtener el nombre del equipo; indícalo en la configuración"}
		}
		name = strings.TrimSpace(host)
	}
	if strings.ContainsAny(name, "\r\n") {
		return []string{"agent.name: no puede ocupar más de una línea"}
	}
	cfg.Agent.Name = name
	return nil
}

func applyBackup(raw rawBackup, cfg *Config) []string {
	var problems []string
	if raw.UseVSS != nil {
		cfg.Backup.UseVSS = *raw.UseVSS
	}
	if len(raw.Sources) == 0 {
		problems = append(problems, "backup.sources: indica al menos una carpeta de origen")
	}
	seen := make(map[string]struct{}, len(raw.Sources))
	for i, source := range raw.Sources {
		field := fmt.Sprintf("backup.sources[%d]", i)
		cleaned := strings.TrimSpace(source)
		if cleaned == "" {
			problems = append(problems, field+": la ruta está vacía")
			continue
		}
		if !absoluteWindowsPath(cleaned) {
			problems = append(problems, field+": la ruta debe ser absoluta, por ejemplo C:\\Datos")
			continue
		}
		key := strings.ToLower(strings.ReplaceAll(cleaned, "/", `\`))
		if _, ok := seen[key]; ok {
			problems = append(problems, field+": la carpeta está repetida")
			continue
		}
		seen[key] = struct{}{}
		cfg.Backup.Sources = append(cfg.Backup.Sources, cleaned)
	}
	for i, pattern := range raw.Excludes {
		cleaned := strings.TrimSpace(pattern)
		if cleaned == "" {
			problems = append(problems, fmt.Sprintf("backup.excludes[%d]: el patrón está vacío", i))
			continue
		}
		cfg.Backup.Excludes = append(cfg.Backup.Excludes, cleaned)
	}
	return problems
}

func applyRepository(raw rawRepository, cfg *Config) []string {
	var problems []string
	path := ""
	if raw.Path != nil {
		path = strings.TrimSpace(*raw.Path)
	}
	if path == "" {
		problems = append(problems, `repository.path: indica la carpeta de destino, por ejemplo E:\Copias o \\nas\copias`)
	} else if !absoluteWindowsPath(path) {
		problems = append(problems, `repository.path: la ruta debe ser absoluta o UNC, por ejemplo E:\Copias o \\nas\copias`)
	} else {
		cfg.Repository.Path = path
	}
	if raw.VolumeID != nil {
		id := strings.TrimSpace(*raw.VolumeID)
		if id != "" {
			if !validVolumeID(id) {
				problems = append(problems, "repository.volume_id: el identificador del disco no es válido")
			} else if uncPath(path) {
				problems = append(problems, "repository.volume_id: una carpeta de red no usa identificador de disco")
			} else {
				cfg.Repository.VolumeID = id
			}
		}
	}

	password := ""
	if raw.PasswordEncrypted != nil {
		password = strings.TrimSpace(*raw.PasswordEncrypted)
	}
	if password == "" {
		problems = append(problems, "repository.password_encrypted: la contraseña no está configurada. Se hará con el comando agent set-password")
	} else if !secrets.Valid(password) {
		problems = append(problems, "repository.password_encrypted: el valor no está cifrado. Ejecuta agent set-password en este equipo; no escribas la contraseña en claro")
	} else {
		cfg.Repository.PasswordEncrypted = password
	}
	return problems
}

func applySchedule(raw rawSchedule, cfg *Config) []string {
	var problems []string
	if raw.RunMissedOnStartup != nil {
		cfg.Schedule.RunMissedOnStartup = *raw.RunMissedOnStartup
	}
	if raw.MaxRetries != nil {
		cfg.Schedule.MaxRetries = *raw.MaxRetries
	}
	if raw.RetryDelayMinutes != nil {
		cfg.Schedule.RetryDelayMinutes = *raw.RetryDelayMinutes
	}
	if raw.StaleAfterDays != nil {
		cfg.Schedule.StaleAfterDays = *raw.StaleAfterDays
	}
	if cfg.Schedule.StaleAfterDays < 1 {
		problems = append(problems, "schedule.stale_after_days: debe ser al menos 1")
	}
	if raw.Weekdays != nil {
		if len(raw.Weekdays) == 0 {
			problems = append(problems, "schedule.weekdays: indica al menos un día (1 es lunes y 7 es domingo)")
		}
		seenDay := map[int]struct{}{}
		for _, day := range raw.Weekdays {
			if day < 1 || day > 7 {
				problems = append(problems, "schedule.weekdays: usa números del 1 (lunes) al 7 (domingo)")
				continue
			}
			if _, dup := seenDay[day]; dup {
				problems = append(problems, fmt.Sprintf("schedule.weekdays: el día %d está repetido", day))
				continue
			}
			seenDay[day] = struct{}{}
			cfg.Schedule.Weekdays = append(cfg.Schedule.Weekdays, day)
		}
	}
	if cfg.Schedule.MaxRetries < 0 {
		problems = append(problems, "schedule.max_retries: no puede ser negativo")
	}
	if cfg.Schedule.MaxRetries > 0 && cfg.Schedule.RetryDelayMinutes < 1 {
		problems = append(problems, "schedule.retry_delay_minutes: debe ser al menos 1 si hay reintentos")
	}
	if cfg.Schedule.RetryDelayMinutes < 0 {
		problems = append(problems, "schedule.retry_delay_minutes: no puede ser negativo")
	}

	if len(raw.Times) == 0 {
		problems = append(problems, "schedule.times: indica al menos una hora (formato HH:MM)")
		return problems
	}
	seen := make(map[string]struct{}, len(raw.Times))
	for i, value := range raw.Times {
		field := fmt.Sprintf("schedule.times[%d]", i)
		normalized, ok := normalizeTime(value)
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: hora no válida %q (usa HH:MM, de 00:00 a 23:59)", field, strings.TrimSpace(value)))
			continue
		}
		if _, dup := seen[normalized]; dup {
			problems = append(problems, fmt.Sprintf("%s: la hora %s está repetida", field, normalized))
			continue
		}
		seen[normalized] = struct{}{}
		cfg.Schedule.Times = append(cfg.Schedule.Times, normalized)
	}
	return problems
}

func applyRetention(raw rawRetention, cfg *Config) []string {
	var problems []string
	if raw.KeepDaily != nil {
		cfg.Retention.KeepDaily = *raw.KeepDaily
	}
	if raw.KeepWeekly != nil {
		cfg.Retention.KeepWeekly = *raw.KeepWeekly
	}
	if raw.KeepMonthly != nil {
		cfg.Retention.KeepMonthly = *raw.KeepMonthly
	}
	if raw.KeepYearly != nil {
		cfg.Retention.KeepYearly = *raw.KeepYearly
	}
	if raw.Prune != nil {
		cfg.Retention.Prune = *raw.Prune
	}
	if cfg.Retention.KeepDaily < 0 {
		problems = append(problems, "retention.keep_daily: no puede ser negativo")
	}
	if cfg.Retention.KeepWeekly < 0 {
		problems = append(problems, "retention.keep_weekly: no puede ser negativo")
	}
	if cfg.Retention.KeepMonthly < 0 {
		problems = append(problems, "retention.keep_monthly: no puede ser negativo")
	}
	if cfg.Retention.KeepYearly < 0 {
		problems = append(problems, "retention.keep_yearly: no puede ser negativo")
	}
	if cfg.Retention.KeepDaily == 0 && cfg.Retention.KeepWeekly == 0 && cfg.Retention.KeepMonthly == 0 && cfg.Retention.KeepYearly == 0 {
		problems = append(problems, "retention: la política no conserva ninguna copia")
	}
	return problems
}

func applyVerify(raw rawVerify, cfg *Config) []string {
	var problems []string
	if raw.EveryDays != nil {
		cfg.Verify.EveryDays = *raw.EveryDays
	}
	if cfg.Verify.EveryDays < 1 {
		problems = append(problems, "verify.every_days: debe ser al menos 1")
	}
	if raw.ReadDataSubset != nil {
		subset := strings.TrimSpace(*raw.ReadDataSubset)
		if !validReadDataSubset(subset) {
			problems = append(problems, fmt.Sprintf("verify.read_data_subset: valor no válido %q (usa un porcentaje, n/t o un tamaño, por ejemplo 5%%, 1/7 o 10G)", subset))
		} else {
			cfg.Verify.ReadDataSubset = subset
		}
	}
	return problems
}

func applyPanel(raw rawPanel, cfg *Config) []string {
	if raw.Port == nil {
		return nil
	}
	if *raw.Port < 1 || *raw.Port > 65535 {
		return []string{"panel.port: debe estar entre 1 y 65535"}
	}
	cfg.Panel.Port = *raw.Port
	return nil
}

func normalizeTime(value string) (string, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return "", false
	}
	hour, errH := strconv.Atoi(parts[0])
	minute, errM := strconv.Atoi(parts[1])
	if errH != nil || errM != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return "", false
	}
	return fmt.Sprintf("%02d:%02d", hour, minute), true
}

func validVolumeID(id string) bool {
	return volumeIDPattern.MatchString(id)
}

func uncPath(path string) bool {
	slash := strings.ReplaceAll(path, "/", `\`)
	return strings.HasPrefix(slash, `\\`) && !strings.HasPrefix(strings.ToLower(slash), `\\?\`)
}

func absoluteWindowsPath(path string) bool {
	if strings.ContainsAny(path, "\r\n") {
		return false
	}
	if len(path) >= 3 && isDriveLetter(rune(path[0])) && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		return true
	}
	slash := strings.ReplaceAll(path, "/", `\`)
	if !strings.HasPrefix(slash, `\\`) {
		return false
	}
	rest := strings.TrimPrefix(slash, `\\`)
	host, tail, ok := strings.Cut(rest, `\`)
	if !ok || host == "" || strings.Contains(host, ":") {
		return false
	}
	share, _, _ := strings.Cut(tail, `\`)
	return share != ""
}

func isDriveLetter(r rune) bool {
	return unicode.IsLetter(r) && r <= unicode.MaxASCII
}

// validReadDataSubset acepta los tres formatos de restic check --read-data-subset:
// un porcentaje (5%, 2.5%), una fracción (1/7) o un tamaño (10G, 500M, 50K).
func validReadDataSubset(value string) bool {
	if value == "" {
		return false
	}
	if strings.HasSuffix(value, "%") {
		n, err := strconv.ParseFloat(strings.TrimSuffix(value, "%"), 64)
		return err == nil && n > 0 && n <= 100
	}
	if strings.Contains(value, "/") {
		left, right, ok := strings.Cut(value, "/")
		if !ok || strings.Contains(right, "/") {
			return false
		}
		n, errN := strconv.Atoi(left)
		t, errT := strconv.Atoi(right)
		return errN == nil && errT == nil && n >= 1 && t >= 1 && n <= t
	}
	if len(value) < 2 {
		return false
	}
	switch value[len(value)-1] {
	case 'k', 'K', 'm', 'M', 'g', 'G', 't', 'T':
	default:
		return false
	}
	n, err := strconv.ParseUint(value[:len(value)-1], 10, 64)
	return err == nil && n >= 1
}
