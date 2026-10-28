param([string]$Path,[string]$OkPath,[string]$Title)
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
[System.Windows.Forms.Application]::EnableVisualStyles()
$form = New-Object System.Windows.Forms.Form
$form.Text = "Текст этикетки - $Title"
$form.StartPosition = 'CenterScreen'
$form.Size = New-Object System.Drawing.Size(700,330)
$form.MinimumSize = New-Object System.Drawing.Size(620,300)
$info = New-Object System.Windows.Forms.Label
$info.Location = New-Object System.Drawing.Point(16,14)
$info.Size = New-Object System.Drawing.Size(650,28)
$info.Text = 'Введите одну или две строки. Enter создаёт вторую строку.'
$form.Controls.Add($info)
$tb = New-Object System.Windows.Forms.TextBox
$tb.Multiline = $true
$tb.AcceptsReturn = $true
$tb.ScrollBars = 'Vertical'
$tb.Font = New-Object System.Drawing.Font('Segoe UI',14)
$tb.Location = New-Object System.Drawing.Point(16,48)
$tb.Size = New-Object System.Drawing.Size(650,130)
$tb.Anchor = 'Top,Left,Right'
$tb.Text = [IO.File]::ReadAllText($Path,[Text.Encoding]::UTF8)
$form.Controls.Add($tb)
$status = New-Object System.Windows.Forms.Label
$status.Location = New-Object System.Drawing.Point(16,188)
$status.Size = New-Object System.Drawing.Size(650,26)
$form.Controls.Add($status)
$save = New-Object System.Windows.Forms.Button
$save.Text = 'Сохранить'
$save.Location = New-Object System.Drawing.Point(16,225)
$save.Size = New-Object System.Drawing.Size(120,36)
$form.Controls.Add($save)
$cancel = New-Object System.Windows.Forms.Button
$cancel.Text = 'Отмена'
$cancel.Location = New-Object System.Drawing.Point(146,225)
$cancel.Size = New-Object System.Drawing.Size(120,36)
$form.Controls.Add($cancel)
$update = {
  $normalized = $tb.Text -replace "`r`n","`n" -replace "`r","`n"
  $lines = @($normalized -split "`n",-1)
  while ($lines.Count -gt 1 -and $lines[-1] -eq '') { $lines = $lines[0..($lines.Count-2)] }
  $n = [Math]::Max(1,$lines.Count)
  if ($n -le 2) { $status.ForeColor=[Drawing.Color]::DarkGreen; $status.Text="Строк: $n из 2" }
  else { $status.ForeColor=[Drawing.Color]::DarkRed; $status.Text="Строк: $n - максимум 2" }
}
$tb.Add_TextChanged($update)
$save.Add_Click({
  $normalized = $tb.Text -replace "`r`n","`n" -replace "`r","`n"
  $lines = @($normalized -split "`n",-1)
  while ($lines.Count -gt 1 -and $lines[-1] -eq '') { $lines = $lines[0..($lines.Count-2)] }
  if ($lines.Count -gt 2) { [Windows.Forms.MessageBox]::Show('Допускается максимум две строки.','MSK Zebra Label Studio'); return }
  [IO.File]::WriteAllText($Path,($lines -join "`n"),(New-Object Text.UTF8Encoding($false)))
  [IO.File]::WriteAllText($OkPath,'ok')
  $form.Close()
})
$cancel.Add_Click({ $form.Close() })
$form.AcceptButton = $save
& $update
$tb.SelectionStart=$tb.TextLength
$tb.Focus()
[void]$form.ShowDialog()