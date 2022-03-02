$global:RedisId = $null
$global:DiscId = $null
$global:DmsgServer1Id = $null
$global:DmsgServer2Id = $null
$global:PtyHost1Id = $null
$global:PtyHost2Id = $null

# CSV-NotEmpty checks if the integration-pids are empty
function CSV-NotEmpty {
    try {
        $processList = @(Import-Csv ".\integration\integration-pids.csv")
        return $processList.Length -gt 0
    } catch {
        return 0
    }
}

function Start-All {
    $r = Start-Process -PassThru powershell -Argument "`$host.UI.RawUI.WindowTitle = 'redis'; redis-server | Out-Host; Read-Host"
    $global:RedisId = $r.Id
    Start-Sleep -Seconds 4
    $disc = Start-Process -PassThru powershell -Argument "`$host.UI.RawUI.WindowTitle = 'dmsg-discovery'; .\bin\dmsg-discovery.exe -t | Out-Host; Read-Host"
    $global:DiscId = $disc.Id
    Start-Sleep -Seconds 4
    $srv1 = Start-Process -PassThru powershell -Argument "`$host.ui.RawUI.WindowTitle = 'dmsg-server-1'; .\bin\dmsg-server.exe .\integration\configs\dmsgserver1.json | Out-Host; Read-Host"
    $global:DmsgServer1Id = $srv1.Id
    $srv2 = Start-Process -PassThru powershell "`$host.UI.RawUI.WindowTitle = 'dmsg-server-2'; .\bin\dmsg-server.exe .\integration\configs\dmsgserver2.json | Out-Host; Read-Host"
    $global:DmsgServer2Id = $srv2.Id
    Start-Sleep -Seconds 5
    $pty1 = Start-Process -PassThru powershell "`$host.UI.RawUI.WindowTitle = 'dmsgpty-host-1'; .\bin\dmsgpty-host.exe -c .\integration\configs\dmsgptyhost1_windows.json | Out-Host; Read-Host"
    $global:PtyHost1Id = $pty1.Id
    $pty2 = Start-Process -PassThru powershell "`$host.UI.RawUI.WindowTitle = 'dmsgpty-host-2'; .\bin\dmsgpty-host.exe -c .\integration\configs\dmsgptyhost2_windows.json | Out-Host; Read-Host"
    $global:PtyHost2Id = $pty2.Id
}

function Export-Pid-Csv {
    @(
        [PSCustomObject]@{
            Name = 'Redis'
            Pid = $global:RedisId
        }
        [PSCustomObject]@{
            Name = 'Dmsg-Discovery'
            Pid = $global:DiscId
        }
        [PSCustomObject]@{
            Name = "Dmsg-Server1"
            Pid = $global:DmsgServer1Id
        }
        [PSCustomObject]@{
            Name = "Dmsg-Server2"
            Pid = $global:DmsgServer2Id
        }
        [PSCustomObject]@{
            Name = "DmsgPty-Host1"
            Pid = $global:PtyHost1Id
        }
        [PSCustomObject]@{
            Name = "DmsgPty-Host2"
            Pid = $global:PtyHost2Id
        }
    ) | Export-Csv ".\integration\integration-pids.csv"
}

function Stop-All {
    $process_list = Import-Csv ".\integration\integration-pids.csv"
    $process_list | ForEach-Object {
        foreach ($proc in $_.PsObject.Properties) {
            Write-Host "Killing $_.Name"
            Kill-Tree $_.Pid
        }
    }
    Remove-Item -Path ".\integration\integration-pids.csv" -Force
}

function Print-Usage {
    Write-Host ".\integration\integration.ps1 [start | stop]"
}

function Kill-Tree {
    Param([int]$ppid)
    Get-CimInstance Win32_Process | Where-Object { $_.ParentProcessId -eq $ppid } | ForEach-Object { Kill-Tree $_.ProcessId }
    Stop-Process -Id $ppid -ErrorAction Ignore
}


$exists = CSV-NotEmpty
switch ($args[0]) {
    start {
        if (!$exists) {
            Start-All
            Export-Pid-Csv
        } else {
            Write-Error -Message "Integration Env already started" -Category InvalidOperation
        }
    }
    stop {
        if ($exists) {
            Stop-All
        } else {
            Write-Error -Message "Integration Env isn't started" -Category InvalidOperation
        }
    }
    default {
        Print-Usage
    }
}
