package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func promptCredentials(host string) (username, password string, ok bool, err error) {
	logger.Infof("Запрос учетных данных для: %s", host)

	// PowerShell скрипт с графическим интерфейсом
	script := fmt.Sprintf(`
    Add-Type -AssemblyName System.Windows.Forms
    Add-Type -AssemblyName System.Drawing
    
    $form = New-Object System.Windows.Forms.Form
    $form.Text = "WebDAV Drive - Вход"
    $form.Size = New-Object System.Drawing.Size(400, 220)
    $form.StartPosition = "CenterScreen"
    $form.TopMost = $true
    $form.FormBorderStyle = "FixedDialog"
    $form.MaximizeBox = $false
    $form.MinimizeBox = $false
    
    # Информация о сервере
    $label = New-Object System.Windows.Forms.Label
    $label.Location = New-Object System.Drawing.Point(10, 10)
    $label.Size = New-Object System.Drawing.Size(360, 40)
    $label.Text = "Сервер: %s"
    
    # Поле для имени пользователя
    $userLabel = New-Object System.Windows.Forms.Label
    $userLabel.Location = New-Object System.Drawing.Point(10, 60)
    $userLabel.Size = New-Object System.Drawing.Size(120, 20)
    $userLabel.Text = "Имя пользователя:"
    
    $userBox = New-Object System.Windows.Forms.TextBox
    $userBox.Location = New-Object System.Drawing.Point(140, 60)
    $userBox.Size = New-Object System.Drawing.Size(200, 20)
    
    # Поле для пароля
    $passLabel = New-Object System.Windows.Forms.Label
    $passLabel.Location = New-Object System.Drawing.Point(10, 90)
    $passLabel.Size = New-Object System.Drawing.Size(120, 20)
    $passLabel.Text = "Пароль:"
    
    $passBox = New-Object System.Windows.Forms.TextBox
    $passBox.Location = New-Object System.Drawing.Point(140, 90)
    $passBox.Size = New-Object System.Drawing.Size(200, 20)
    $passBox.PasswordChar = '*'
    
    # Кнопки
    $okButton = New-Object System.Windows.Forms.Button
    $okButton.Location = New-Object System.Drawing.Point(140, 130)
    $okButton.Size = New-Object System.Drawing.Size(75, 30)
    $okButton.Text = "OK"
    
    $cancelButton = New-Object System.Windows.Forms.Button
    $cancelButton.Location = New-Object System.Drawing.Point(225, 130)
    $cancelButton.Size = New-Object System.Drawing.Size(75, 30)
    $cancelButton.Text = "Отмена"
    
    # Добавляем элементы
    $form.Controls.Add($label)
    $form.Controls.Add($userLabel)
    $form.Controls.Add($userBox)
    $form.Controls.Add($passLabel)
    $form.Controls.Add($passBox)
    $form.Controls.Add($okButton)
    $form.Controls.Add($cancelButton)
    
    # Результат
    $script:result = $null
    
    $okButton.Add_Click({
        if ($userBox.Text -ne "" -and $passBox.Text -ne "") {
            $script:result = @($userBox.Text, $passBox.Text)
            $form.Close()
        } else {
            [System.Windows.Forms.MessageBox]::Show("Заполните все поля", "Ошибка", "OK", "Error")
        }
    })
    
    $cancelButton.Add_Click({
        $form.Close()
    })
    
    # Фокус
    $form.Add_Shown({
        $userBox.Focus()
    })
    
    # Показываем форму
    $form.ShowDialog() | Out-Null
    
    # Возвращаем результат
    if ($script:result -ne $null) {
        Write-Output $script:result[0]
        Write-Output $script:result[1]
    }
    `, host)

	// Запускаем PowerShell
	cmd := exec.Command("powershell",
		"-ExecutionPolicy", "Bypass",
		"-NoProfile",
		"-WindowStyle", "Normal",
		"-Command", script)

	output, err := cmd.Output()

	if err != nil {
		// Если пользователь закрыл окно - это не ошибка
		if exitErr, ok := err.(*exec.ExitError); ok {
			logger.Infof("PowerShell завершился с кодом: %d", exitErr.ExitCode())
		}
		return "", "", false, nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) >= 2 {
		username = strings.TrimSpace(lines[0])
		password = strings.TrimSpace(lines[1])
		ok = username != "" && password != ""
		return username, password, ok, nil
	}

	return "", "", false, nil
}
