Option Explicit
Dim shell, files, root, executable
Set shell = CreateObject("WScript.Shell")
Set files = CreateObject("Scripting.FileSystemObject")
root = files.GetParentFolderName(WScript.ScriptFullName)
executable = files.BuildPath(root, "bin\poker-simulator.exe")
If Not files.FileExists(executable) Then
  executable = files.BuildPath(root, "poker-simulator.exe")
End If
If files.FileExists(executable) Then
  shell.Run Chr(34) & executable & Chr(34), 0, False
Else
  MsgBox "Build the application first with scripts\build.ps1, or download a Windows release ZIP.", 48, "Poker Superpower Simulator"
End If
