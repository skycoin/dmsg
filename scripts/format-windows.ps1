# GoImports format for windows

Get-ChildItem -Directory | where Name -NotMatch vendor | % { Get-ChildItem $_ -Recurse -Include *.go } | % {goimports -w -local github.com/skycoin/dmsg $_ }
Get-ChildItem | Where-Object { $_.Extension -match '.go' } | % { goimports -w -local github.com/skycoin/dmsg $_ }