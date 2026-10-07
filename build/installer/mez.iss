; The mez installer for Windows (phase 12, Task 2; ADR-265).
;
; Build it with `make installer`, after `make dist`. By hand:
;
;   ISCC /DVersion=1.0.0 /DBinary=..\..\dist\mez_1.0.0_windows_amd64.exe mez.iss
;
; It copies mez.exe, registers it as a Windows service under its own
; account (ADR-264), lets the LAN through the firewall, puts a full-screen
; shortcut on the desktop and starts the server. Everything that touches the
; service is done by mez.exe itself, so the installer and a terminal cannot
; disagree about how a service is installed.

#ifndef Version
  #define Version "dev"
#endif
#ifndef Binary
  #error Binary is not set: pass /DBinary=path\to\mez_windows_amd64.exe
#endif

; Where the server listens unless config.toml says otherwise. The shortcut
; and the wait for the server use it; the firewall rule does not, since it
; names the program.
#define Url "http://localhost:7777"
#define HealthUrl "http://127.0.0.1:7777/api/v1/health"
#define FirewallRule "mez"

[Setup]
; Never change AppId: it is how an upgrade finds the install before it.
AppId={{6F1E7C52-3B0A-4E8D-9A41-7C2D5B90E3A6}
AppName=mez
AppVersion={#Version}
AppVerName=mez {#Version}
AppPublisher=mez
AppPublisherURL=https://github.com/byjoellobo/mez
AppSupportURL=https://github.com/byjoellobo/mez/issues
DefaultDirName={autopf}\mez
; No questions an owner cannot answer: the folder and the Start menu group
; are decided.
DisableDirPage=yes
DisableProgramGroupPage=yes
DisableWelcomePage=no
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
OutputDir=..\..\dist
OutputBaseFilename=mez_{#Version}_windows_amd64_setup
SetupIconFile=mez.ico
UninstallDisplayIcon={app}\mez.ico
UninstallDisplayName=mez
WizardStyle=modern
Compression=lzma2
SolidCompression=yes
; The service is stopped by this script before the files are replaced, so
; there is nothing for the Restart Manager to close.
CloseApplications=no
#ifdef Sign
; `make installer MEZ_SIGNTOOL="signtool sign ... $f"` names the tool.
SignTool=signtool
SignedUninstaller=yes
#endif

[Messages]
WelcomeLabel2=This installs mez on this computer and starts it. It will start by itself each time the computer is turned on.%n%nTills, tablets and kitchen screens on the same network open it in a browser. Nothing is sent to the internet.
FinishedLabel=mez is installed and running on this computer.%n%nThe first time, leave "Open mez now" ticked: it starts the setup of your venue, which can only be started from here. After that, open mez from its shortcut on the desktop. Other devices on this network can open it in a browser.

[Tasks]
Name: "desktopicon"; Description: "Put a mez shortcut on the desktop (opens the till full screen)"
Name: "firewall"; Description: "Let tills, tablets and kitchen screens on this network connect"

[Files]
Source: "{#Binary}"; DestDir: "{app}"; DestName: "mez.exe"; Flags: ignoreversion
Source: "mez.ico"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autodesktop}\mez"; Filename: "{code:BrowserPath}"; Parameters: "{code:BrowserArgs}"; IconFilename: "{app}\mez.ico"; Comment: "Open the mez till"; Tasks: desktopicon
Name: "{autoprograms}\mez"; Filename: "{code:BrowserPath}"; Parameters: "{code:BrowserArgs}"; IconFilename: "{app}\mez.ico"; Comment: "Open the mez till"

[Run]
; The same browser as the shortcut, once, as the person who ran the installer
; and not as the administrator it was raised to. On a new machine it is opened
; at the setup link, which carries the token that lets the venue be made
; (ADR-270): only this installer, raised, can read it.
Filename: "{code:BrowserPath}"; Parameters: "{code:FirstRunArgs}"; Description: "Open mez now"; Flags: postinstall nowait skipifsilent runasoriginaluser

[Code]
const
  ServiceLog = 'Look in Event Viewer, under Windows Logs, Application, for the source "mez": it says in words why.';

// MezExe is the installed binary.
function MezExe: String;
begin
  Result := ExpandConstant('{app}\mez.exe');
end;

// FirstFile answers the first of the paths that exists, or ''.
function FirstFile(Paths: array of String): String;
var
  I: Integer;
begin
  Result := '';
  for I := 0 to GetArrayLength(Paths) - 1 do
    if FileExists(ExpandConstant(Paths[I])) then
    begin
      Result := ExpandConstant(Paths[I]);
      Exit;
    end;
end;

function EdgePath: String;
begin
  Result := FirstFile([
    '{commonpf32}\Microsoft\Edge\Application\msedge.exe',
    '{commonpf64}\Microsoft\Edge\Application\msedge.exe']);
end;

function ChromePath: String;
begin
  Result := FirstFile([
    '{commonpf64}\Google\Chrome\Application\chrome.exe',
    '{commonpf32}\Google\Chrome\Application\chrome.exe',
    '{localappdata}\Google\Chrome\Application\chrome.exe']);
end;

// BrowserPath is what the shortcut starts: Edge, which every Windows 10 and
// 11 has, then Chrome, then whatever opens a web address.
function BrowserPath(Param: String): String;
begin
  Result := EdgePath;
  if Result = '' then
    Result := ChromePath;
  if Result = '' then
    Result := ExpandConstant('{win}\explorer.exe');
end;

// ArgsFor opens an address as an app, full screen, with no address bar.
// Edge wants its kiosk type said; with neither browser the address alone is
// handed to Windows.
function ArgsFor(Address: String): String;
begin
  if EdgePath <> '' then
    Result := '--app=' + Address + ' --kiosk --edge-kiosk-type=fullscreen'
  else if ChromePath <> '' then
    Result := '--app=' + Address + ' --kiosk'
  else
    Result := Address;
end;

// BrowserArgs is what the shortcut opens: the till.
function BrowserArgs(Param: String): String;
begin
  Result := ArgsFor('{#Url}');
end;

// Said joins what a command printed, so that a failure is shown in the
// command's own words.
function Said(Output: TExecOutput): String;
var
  I: Integer;
begin
  Result := '';
  for I := 0 to GetArrayLength(Output.StdErr) - 1 do
    Result := Result + Output.StdErr[I] + #13#10;
  for I := 0 to GetArrayLength(Output.StdOut) - 1 do
    Result := Result + Output.StdOut[I] + #13#10;
end;

// Mez runs `mez.exe <Params>` and reports whether it succeeded. What it
// printed comes back in Output.
function Mez(Params: String; var Output: String): Boolean;
var
  Code: Integer;
  Captured: TExecOutput;
begin
  Output := '';
  Result := ExecAndCaptureOutput(MezExe, Params, ExpandConstant('{app}'), SW_HIDE, ewWaitUntilTerminated, Code, Captured);
  if Result then
  begin
    Output := Said(Captured);
    Result := Code = 0;
  end
  else
    Output := SysErrorMessage(Code);
end;

// SetupLink asks mez.exe where to open it. On a machine with no venue yet
// that is the setup wizard's address with the setup token in it, which is
// kept where only an administrator can read it; otherwise it is the till's.
// The installer does not read the token itself: `mez setup-link` is the one
// thing that knows where it is and what it looks like. Anything but an
// http address on localhost is not believed, and the till's is used.
function SetupLink: String;
var
  Code, I: Integer;
  Captured: TExecOutput;
  Line: String;
begin
  Result := '{#Url}';
  if not ExecAndCaptureOutput(MezExe, 'setup-link', ExpandConstant('{app}'), SW_HIDE, ewWaitUntilTerminated, Code, Captured) then
    Exit;
  if Code <> 0 then
    Exit;
  for I := 0 to GetArrayLength(Captured.StdOut) - 1 do
  begin
    Line := Trim(Captured.StdOut[I]);
    if (Pos('http://localhost', Line) = 1) and (Pos(' ', Line) = 0) and (Pos('"', Line) = 0) then
    begin
      Result := Line;
      Exit;
    end;
  end;
end;

// FirstRunArgs is what "Open mez now" opens on the installer's last page.
function FirstRunArgs(Param: String): String;
begin
  Result := ArgsFor(SetupLink);
end;

procedure Netsh(Params: String);
var
  Code: Integer;
begin
  Exec(ExpandConstant('{sys}\netsh.exe'), 'advfirewall firewall ' + Params, '', SW_HIDE, ewWaitUntilTerminated, Code);
end;

// ServerIsUp asks the server for its health.
function ServerIsUp: Boolean;
var
  Http: Variant;
begin
  Result := False;
  try
    Http := CreateOleObject('WinHttp.WinHttpRequest.5.1');
    Http.SetTimeouts(1000, 1000, 1000, 1000);
    Http.Open('GET', '{#HealthUrl}', False);
    Http.Send('');
    Result := Http.Status = 200;
  except
  end;
end;

// WaitForServer gives the server twenty seconds to answer. Starting a
// service returns once Windows has been asked, not once the till is there,
// and a browser opened too soon shows an error to someone who has just been
// told it worked.
function WaitForServer: Boolean;
var
  Tries: Integer;
begin
  Result := False;
  for Tries := 1 to 40 do
  begin
    if ServerIsUp then
    begin
      Result := True;
      Exit;
    end;
    Sleep(500);
  end;
end;

// PrepareToInstall takes an install before this one out of the way. The
// service holds mez.exe open, and the new version may be installed with
// different settings, so the old service is stopped and removed by the old
// binary and registered again by the new one. The data is not touched.
function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  Output: String;
begin
  Result := '';
  if FileExists(MezExe) then
    Mez('service uninstall', Output); // not installed is not a failure here
end;

procedure InstallService;
var
  Output: String;
begin
  WizardForm.StatusLabel.Caption := 'Setting mez to start with Windows...';
  if not Mez('service install', Output) then
  begin
    MsgBox('mez was copied to this computer, but could not be set to start with Windows:' + #13#10#13#10 + Output, mbError, MB_OK);
    Exit;
  end;

  if WizardIsTaskSelected('firewall') then
  begin
    // By program and not by port, so a port changed in config.toml is
    // still let through. Private and domain networks only: on a network
    // Windows calls Public, other devices are kept out.
    Netsh('delete rule name="{#FirewallRule}"');
    Netsh('add rule name="{#FirewallRule}" dir=in action=allow protocol=TCP profile=private,domain program="' + MezExe + '"');
  end;

  WizardForm.StatusLabel.Caption := 'Starting mez...';
  if not Mez('service start', Output) then
  begin
    MsgBox('mez is installed, but did not start:' + #13#10#13#10 + Output + #13#10 + ServiceLog, mbError, MB_OK);
    Exit;
  end;
  if not WaitForServer then
    MsgBox('mez is installed and was started, but is not answering yet.' + #13#10#13#10 +
      'If the shortcut shows an error, wait a minute and try again. If it still does: ' + ServiceLog, mbInformation, MB_OK);
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
    InstallService;
end;

// The uninstaller stops and removes the service while mez.exe is still
// there to do it, then asks about the data. Keeping it is the answer that
// a slip of the hand gives.
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Output, Data: String;
begin
  if CurUninstallStep = usUninstall then
  begin
    Mez('service uninstall', Output);
    Netsh('delete rule name="{#FirewallRule}"');
  end;

  if CurUninstallStep = usPostUninstall then
  begin
    Data := ExpandConstant('{commonappdata}\mez');
    if (not DirExists(Data)) or UninstallSilent then
      Exit;
    if MsgBox('mez has been removed.' + #13#10#13#10 +
      'The venue''s data is still on this computer, in ' + Data + ': every sale, invoice, the menu, the staff and the logs. ' +
      'Installing mez again will pick it up where it stopped.' + #13#10#13#10 +
      'Delete the data as well?', mbConfirmation, MB_YESNO or MB_DEFBUTTON2) <> IDYES then
      Exit;
    if MsgBox('This deletes every sale and invoice this venue has recorded. It cannot be undone, and there is no copy unless you made one.' + #13#10#13#10 +
      'Delete ' + Data + '?', mbError, MB_YESNO or MB_DEFBUTTON2) <> IDYES then
      Exit;
    if not DelTree(Data, True, True, True) then
      MsgBox('Some of the data could not be deleted. What is left is in ' + Data + '.', mbInformation, MB_OK);
  end;
end;
