; webdav-drive.iss — Inno Setup script
[Setup]
AppName=WebDAV Drive
AppVersion=1.0.0
DefaultDirName={commonpf}\WebDAV Drive
DefaultGroupName=WebDAV Drive
OutputDir=.
OutputBaseFilename=webdav-drive-setup
Compression=lzma
SolidCompression=yes
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
SetupIconFile=setup-icon.ico
UninstallDisplayIcon={app}\webdav-drive.exe
LicenseFile=..\LICENSE
UsedUserAreasWarning=no

[Files]
Source: "..\src\webdav-drive.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\WebDAV Drive"; Filename: "{app}\webdav-drive.exe"
Name: "{userstartup}\WebDAV Drive"; Filename: "{app}\webdav-drive.exe"

[Run]
Filename: "{app}\webdav-drive.exe"; Parameters: "--first-run"; Description: "Запустить WebDAV Drive"; Flags: postinstall nowait skipifsilent

[Registry]
; Разрешить HTTPS WebDAV
Root: HKLM; Subkey: "SYSTEM\CurrentControlSet\Services\WebClient\Parameters"; ValueType: dword; ValueName: "BasicAuthLevel"; ValueData: "2"

[Code]
// Включить и запустить WebClient
procedure EnableWebClient();
var
  ResultCode: Integer;
begin
  if not ShellExec('', ExpandConstant('sc'), 'config webclient start= auto', '', SW_HIDE, ewWaitUntilTerminated, ResultCode) then
  begin
    MsgBox('Не удалось настроить службу WebClient (автозапуск).', mbError, MB_OK);
  end;
  if not ShellExec('', ExpandConstant('net'), 'start webclient', '', SW_HIDE, ewWaitUntilTerminated, ResultCode) then
  begin
    // Игнорируем, если уже запущена
  end;
end;

// Отключить сетевой диск при удалении
procedure UninstallWebDAVDisk();
var
  ConfigPath: String;
  ConfigContent: AnsiString;
  DriveLetter: String;
  StartPos, EndPos, ResultCode: Integer;
begin
  ConfigPath := ExpandConstant('{localappdata}') + '\WebDAV Drive\config.json';
  if not LoadStringFromFile(ConfigPath, ConfigContent) then
    exit;

  StartPos := Pos('drive_letter:', ConfigContent);
  if StartPos = 0 then exit;

  StartPos := StartPos + Length('drive_letter:');
  while (StartPos <= Length(ConfigContent)) and (ConfigContent[StartPos] <= ' ') do
    Inc(StartPos);

  EndPos := StartPos;
  while (EndPos <= Length(ConfigContent)) and (ConfigContent[EndPos] > ' ') do
    Inc(EndPos);

  if EndPos > StartPos then
  begin
    DriveLetter := Copy(ConfigContent, StartPos, EndPos - StartPos);
    StringChange(DriveLetter, '"', '');
    StringChange(DriveLetter, '''', '');
    DriveLetter := Trim(DriveLetter);

    if (Length(DriveLetter) >= 2) and (DriveLetter[Length(DriveLetter)] = ':') then
    begin
      Exec(ExpandConstant('{cmd}'), '/c net use ' + DriveLetter + ' /delete /y', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
    end;
  end;
end;

// Удалить учётные данные из Credential Manager
procedure UninstallWebDAVCredentials();
var
  ConfigPath: String;
  ConfigContent: AnsiString;
  WebDAVURL: String;
  StartPos, EndPos, ResultCode: Integer;
begin
  ConfigPath := ExpandConstant('{localappdata}') + '\WebDAV Drive\config.json';
  if not LoadStringFromFile(ConfigPath, ConfigContent) then
    exit;

  StartPos := Pos('webdav_url:', ConfigContent);
  if StartPos = 0 then exit;

  StartPos := StartPos + Length('webdav_url:');
  while (StartPos <= Length(ConfigContent)) and (ConfigContent[StartPos] <= ' ') do
    Inc(StartPos);

  EndPos := StartPos;
  while (EndPos <= Length(ConfigContent)) and (ConfigContent[EndPos] > ' ') do
    Inc(EndPos);

  if EndPos > StartPos then
  begin
    WebDAVURL := Copy(ConfigContent, StartPos, EndPos - StartPos);
    WebDAVURL := Trim(WebDAVURL);

    if WebDAVURL <> '' then
    begin
      Exec(ExpandConstant('{cmd}'), '/c cmdkey /delete:' + WebDAVURL, '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
    end;
  end;
end;

// Удалить папку настроек
procedure RemoveAppData();
begin
  DelTree(ExpandConstant('{localappdata}') + '\WebDAV Drive', True, True, True);
end;

// При установке
procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
  begin
    EnableWebClient();
  end;
end;

// При деинсталляции
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usUninstall then
  begin
    UninstallWebDAVDisk();           // ← ОТКЛЮЧАЕТ СЕТЕВОЙ ДИСК
    UninstallWebDAVCredentials();    // ← УДАЛЯЕТ УЧЁТНЫЕ ДАННЫЕ
    RemoveAppData();                 // ← УДАЛЯЕТ КОНФИГ И ЛОГИ
  end;
end;