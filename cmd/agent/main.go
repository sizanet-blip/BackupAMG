// Command agent es el agente de copias de seguridad para Windows.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"backup-agent/internal/app"
	"backup-agent/internal/config"
	"backup-agent/internal/logging"
	"backup-agent/internal/panel"
	"backup-agent/internal/paths"
	"backup-agent/internal/system"
	"golang.org/x/term"
)

func init() {
	if name := os.Getenv("BACKUP_AGENT_PIPE"); name != "" {
		panel.PipeName = name
	}
}

func main() {
	system.EnableUTF8Console()
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	switch args[0] {
	case "run":
		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "El comando run no admite argumentos.\n\n")
			usage()
			return 2
		}
		return runConsole()
	case "set-password":
		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "El comando set-password no admite argumentos.\n\n")
			usage()
			return 2
		}
		return setPassword()
	case "init-repo":
		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "El comando init-repo no admite argumentos.\n\n")
			usage()
			return 2
		}
		return initRepo()
	case "backup-now":
		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "El comando backup-now no admite argumentos.\n\n")
			usage()
			return 2
		}
		return backupNow()
	case "verify":
		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "El comando verify no admite argumentos.\n\n")
			usage()
			return 2
		}
		return verifyNow()
	case "replicate":
		if len(args) > 2 {
			fmt.Fprintf(os.Stderr, "El comando replicate admite como mucho un identificador.\n\n")
			usage()
			return 2
		}
		return replicateNow(args[1:])
	case "enroll":
		return enrollCommand(args[1:])
	case "enroll-file":
		if len(args) > 2 {
			fmt.Fprintf(os.Stderr, "El comando enroll-file admite como mucho una ruta.\n\n")
			usage()
			return 2
		}
		return enrollFileCommand(args[1:])
	case "install", "uninstall", "start", "stop":
		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "El comando %s no admite argumentos.\n\n", args[0])
			usage()
			return 2
		}
		return manageService(args[0])
	case "secure-data":
		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "El comando secure-data no admite argumentos.\n\n")
			usage()
			return 2
		}
		return secureData()
	case "panel":
		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "El comando panel no admite argumentos.\n\n")
			usage()
			return 2
		}
		return openPanel()
	case "service":
		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "El comando service no admite argumentos.\n\n")
			usage()
			return 2
		}
		return runInstalledService()
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Comando desconocido: %s\n\n", args[0])
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `%s %s

Uso:
  agent run            Ejecuta el agente y hace las copias a las horas configuradas.
  agent set-password   Pide la contraseña del repositorio y la guarda cifrada.
  agent init-repo      Crea el repositorio de restic si todavía no existe.
  agent backup-now     Hace una copia inmediata, aplica la retención y termina.
  agent verify         Comprueba el repositorio ahora.
  agent replicate [id] Pone al día las copias adicionales (todas o la indicada).
  agent enroll [--server URL] <clave>  Da de alta el equipo en la plataforma web (clave de licencia o código de instalación).
  agent enroll-file [ruta]  Da de alta el equipo con alta-backupamg.json.
  agent install        Registra el servicio de Windows. Hace falta administrador.
  agent uninstall      Quita el servicio de Windows.
  agent start          Arranca el servicio.
  agent stop           Detiene el servicio. Si hay una copia en curso, la corta.
  agent secure-data    Deja ProgramData solo para SYSTEM y administradores.
  agent panel          Abre el panel en el navegador. El agente tiene que estar en marcha.
  agent help           Muestra esta ayuda.

Los datos se guardan en %%ProgramData%%\%s.
La configuración es config.yaml dentro de esa carpeta.
Para usar otra carpeta en consola, define la variable %s.
El servicio no usa esa variable: lee siempre la carpeta de ProgramData.
`, app.Name, app.Version, app.Name, app.DataDirEnv)
}

func runConsole() int {
	if err := paths.Ensure(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	logger, err := logging.New(logging.Options{
		LogFile: paths.LogFile(),
		Console: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	defer func() {
		if err := logger.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}()

	logger.Info("Agente iniciado",
		"version", app.Version,
		"mode", "console",
		"data_dir", paths.DataDir(),
	)
	logger.Info("Agente en ejecución. Pulsa Ctrl+C para detenerlo.")
	tidyData(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	code := runLoop(ctx, logger)
	if code == 0 {
		logger.Info("Apagado normal")
	}
	return code
}

func loadLoggedConfig(logger *logging.Logger) (*config.Config, error) {
	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		logger.Error("No se pudo cargar la configuración", "error", err.Error())
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return nil, err
	}
	logger.Info("Configuración cargada",
		"agent", cfg.Agent.Name,
		"sources", len(cfg.Backup.Sources),
		"repository", cfg.Repository.Path,
		"times", strings.Join(cfg.Schedule.Times, ", "),
		"vss", cfg.Backup.UseVSS,
	)
	return cfg, nil
}

func setPassword() int {
	if err := paths.Ensure(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	path := paths.ConfigPath()
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: no se encontró %s.\nCopia configs\\config.example.yaml a esa ruta y ajústala antes de guardar la contraseña.\n", path)
			return 1
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		fmt.Fprintf(os.Stderr, "Error: set-password hay que ejecutarlo en una consola, para poder pedir la contraseña por teclado.\n")
		return 1
	}

	err := config.SavePassword(path, func(label string) (string, error) {
		fmt.Fprint(os.Stderr, label)
		entered, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("no se pudo leer la contraseña: %w", err)
		}
		return string(entered), nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if logger, err := logging.New(logging.Options{LogFile: paths.LogFile(), Console: false}); err == nil {
		logger.Info("Contraseña del repositorio actualizada")
		_ = logger.Close()
	}
	fmt.Fprintf(os.Stderr, "Contraseña guardada en %s\n", path)
	fmt.Fprintf(os.Stderr, "Si se pierde, las copias no se pueden recuperar: restic no permite restablecerla. Guárdala también en un lugar seguro, fuera de este equipo.\n")
	return 0
}
