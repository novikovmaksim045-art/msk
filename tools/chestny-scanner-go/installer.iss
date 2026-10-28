#define MyAppName "Честный знак - сканер КМ"
#define MyAppVersion "1.3.0"
#define MyAppExeName "ChestnyZnakScanner.exe"

[Setup]
AppId={{A4F97689-86D1-428D-81E7-1E8CF3F9352E}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
DefaultDirName={localappdata}\Programs\ChestnyZnakScanner
DefaultGroupName={#MyAppName}
PrivilegesRequired=lowest
OutputDir=installer
OutputBaseFilename=ChestnyZnakScanner_Setup_1.3.0
Compression=lzma2
SolidCompression=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
UninstallDisplayIcon={app}\{#MyAppExeName}
WizardStyle=modern
DisableProgramGroupPage=yes
SetupIconFile=app.ico

[Files]
Source: "dist\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{autoprograms}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "Создать ярлык на рабочем столе"; GroupDescription: "Ярлыки:"; Flags: unchecked

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "Запустить программу"; Flags: nowait postinstall skipifsilent