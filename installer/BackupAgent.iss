; Instalador de BackupAgent.
; El nombre y la versión tienen que coincidir con internal/app/app.go.

#define MyAppName "BackupAgent"
#define MyAppVersion "0.1.0"
#define MyAppPublisher "BackupAgent"
#define MyAppExeName "agent.exe"
#define MyTrayExeName "BackupAgentTray.exe"

[Setup]
AppId={{9F1072EA-11FF-416A-B538-48C0A95DB690}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
OutputDir=..\dist
OutputBaseFilename=BackupAgent-Setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
UninstallDisplayName={#MyAppName}
SetupLogging=yes
LicenseFile=..\docs\THIRD_PARTY_LICENSES.md

[Languages]
Name: "spanish"; MessagesFile: "compiler:Languages\Spanish.isl"

[Files]
Source: "..\bin\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\bin\tray.exe"; DestDir: "{app}"; DestName: "{#MyTrayExeName}"; Flags: ignoreversion
Source: "..\bin\restic.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\docs\THIRD_PARTY_LICENSES.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\configs\config.example.yaml"; DestDir: "{commonappdata}\{#MyAppName}"; DestName: "config.yaml"; Flags: onlyifdoesntexist uninsneveruninstall

[Dirs]
Name: "{commonappdata}\{#MyAppName}"; Flags: uninsneveruninstall
Name: "{commonappdata}\{#MyAppName}\reports"; Flags: uninsneveruninstall
Name: "{commonappdata}\{#MyAppName}\logs"; Flags: uninsneveruninstall

[Registry]
Root: HKLM; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "{#MyAppName}"; ValueData: """{app}\{#MyTrayExeName}"""; Flags: uninsdeletevalue

[UninstallRun]
Filename: "{sys}\taskkill.exe"; Parameters: "/F /IM {#MyTrayExeName}"; Flags: runhidden waituntilterminated; RunOnceId: "StopBackupAgentTray"

[Icons]
Name: "{group}\Consola de {#MyAppName}"; Filename: "{sys}\cmd.exe"; Parameters: "/k cd /d ""{app}"""
Name: "{group}\Desinstalar {#MyAppName}"; Filename: "{uninstallexe}"

[Code]
procedure StopAndRemoveService;
var
  ResultCode: Integer;
begin
  Exec(ExpandConstant('{app}\{#MyAppExeName}'), 'stop', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Exec(ExpandConstant('{app}\{#MyAppExeName}'), 'uninstall', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
end;

procedure StopTray;
var
  ResultCode: Integer;
begin
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /IM {#MyTrayExeName}', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  ResultCode: Integer;
  DataDir: String;
begin
  if CurStep = ssInstall then
  begin
    { El icono y el servicio bloquean los ejecutables mientras siguen abiertos. }
    StopTray;
    StopAndRemoveService;
    Exit;
  end;
  if CurStep <> ssPostInstall then
    Exit;
  DataDir := ExpandConstant('{commonappdata}\{#MyAppName}');
  if not Exec(ExpandConstant('{app}\{#MyAppExeName}'), 'secure-data', '', SW_HIDE, ewWaitUntilTerminated, ResultCode) or (ResultCode <> 0) then
  begin
    MsgBox('No se pudieron restringir los permisos de ' + DataDir + '.' + #13#10 +
      'La contraseña cifrada quedaría al alcance de cualquier usuario de este equipo.',
      mbError, MB_OK);
    Abort;
  end;
  if not Exec(ExpandConstant('{app}\{#MyAppExeName}'), 'install', '', SW_HIDE, ewWaitUntilTerminated, ResultCode) or (ResultCode <> 0) then
  begin
    if not WizardSilent then
      MsgBox('No se pudo registrar el servicio.' + #13#10 +
        'Ábrelo después con: "' + ExpandConstant('{app}\{#MyAppExeName}') + '" install',
        mbError, MB_OK);
    Exit;
  end;
  Exec(ExpandConstant('{app}\{#MyTrayExeName}'), '', '', SW_SHOWNORMAL, ewNoWait, ResultCode);
  if not WizardSilent then
    MsgBox('La instalación ha terminado.' + #13#10 + #13#10 +
      'El icono de la bandeja ya está visible y se abrirá solo al iniciar sesión.' + #13#10 +
      'Ábrelo y sigue el asistente del panel. La contraseña, si se pierde, no se puede recuperar: guárdala también fuera de este equipo.' + #13#10 + #13#10 +
      'El servicio arrancará solo al encender el equipo. Todavía no se ha iniciado.' + #13#10 +
      'Cuando el asistente esté listo, inícialo con: "' + ExpandConstant('{app}\{#MyAppExeName}') + '" start' + #13#10 + #13#10 +
      'La configuración, los informes y los logs se quedan en ' + DataDir + ' aunque desinstales el programa.',
      mbInformation, MB_OK);
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usUninstall then
    StopAndRemoveService;
end;
