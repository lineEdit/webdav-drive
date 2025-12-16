package main

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
)

func promptCredentials(host string) (username, password string, ok bool, err error) {
	logger.Infof("Запрос учетных данных для: %s", host)

	// PowerShell команда для создания графического диалога
	psCommand := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$form = New-Object System.Windows.Forms.Form
$form.Text = 'WebDAV Drive - Вход'
$form.Size = New-Object System.Drawing.Size(400, 220)
$form.StartPosition = 'CenterScreen'
$form.FormBorderStyle = 'FixedDialog'
$form.MaximizeBox = $false
$form.MinimizeBox = $false
$form.TopMost = $true

# Информация о сервере
$serverLabel = New-Object System.Windows.Forms.Label
$serverLabel.Location = New-Object System.Drawing.Point(10, 10)
$serverLabel.Size = New-Object System.Drawing.Size(360, 30)
$serverLabel.Text = 'Сервер: %s'
$serverLabel.Font = New-Object System.Drawing.Font('Arial', 9, [System.Drawing.FontStyle]::Bold)

# Поле для имени пользователя
$userLabel = New-Object System.Windows.Forms.Label
$userLabel.Location = New-Object System.Drawing.Point(10, 50)
$userLabel.Size = New-Object System.Drawing.Size(120, 20)
$userLabel.Text = 'Имя пользователя:'

$userBox = New-Object System.Windows.Forms.TextBox
$userBox.Location = New-Object System.Drawing.Point(140, 50)
$userBox.Size = New-Object System.Drawing.Size(200, 20)

# Поле для пароля
$passLabel = New-Object System.Windows.Forms.Label
$passLabel.Location = New-Object System.Drawing.Point(10, 80)
$passLabel.Size = New-Object System.Drawing.Size(120, 20)
$passLabel.Text = 'Пароль:'

$passBox = New-Object System.Windows.Forms.TextBox
$passBox.Location = New-Object System.Drawing.Point(140, 80)
$passBox.Size = New-Object System.Drawing.Size(200, 20)
$passBox.PasswordChar = '*'

# Кнопки
$okButton = New-Object System.Windows.Forms.Button
$okButton.Location = New-Object System.Drawing.Point(140, 120)
$okButton.Size = New-Object System.Drawing.Size(75, 30)
$okButton.Text = 'Войти'

$cancelButton = New-Object System.Windows.Forms.Button
$cancelButton.Location = New-Object System.Drawing.Point(225, 120)
$cancelButton.Size = New-Object System.Drawing.Size(75, 30)
$cancelButton.Text = 'Отмена'

# Добавляем элементы
$form.Controls.Add($serverLabel)
$form.Controls.Add($userLabel)
$form.Controls.Add($userBox)
$form.Controls.Add($passLabel)
$form.Controls.Add($passBox)
$form.Controls.Add($okButton)
$form.Controls.Add($cancelButton)

# Результат
$script:result = $null

# Обработчик кнопки OK
$okButton.Add_Click({
    $user = $userBox.Text.Trim()
    $pass = $passBox.Text
    
    if ($user -eq '' -or $pass -eq '') {
        [System.Windows.Forms.MessageBox]::Show('Заполните все поля', 'Внимание', 'OK', 'Warning')
        return
    }
    
    # Экранируем специальные символы в пароле для безопасной передачи
    $passEscaped = $pass -replace '([\\''""])', '\$1'
    
    # Используем base64 для безопасной передачи пароля
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($passEscaped)
    $passBase64 = [System.Convert]::ToBase64String($bytes)
    
    $script:result = @($user, $passBase64)
    $form.Close()
})

# Обработчик кнопки Отмена
$cancelButton.Add_Click({
    $form.Close()
})

# Фокус на поле ввода
$form.Add_Shown({
    $userBox.Focus()
})

# Показываем диалог
$form.ShowDialog() | Out-Null

# Возвращаем результат
if ($script:result -ne $null) {
    Write-Output $script:result[0]
    Write-Output $script:result[1]
}
`, host)

	// Запускаем PowerShell процесс
	cmd := exec.Command("powershell",
		"-ExecutionPolicy", "Bypass",
		"-NoProfile",
		"-WindowStyle", "Normal",
		"-Command", psCommand)

	// Получаем вывод
	output, err := cmd.Output()

	// Обрабатываем результат
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			logger.Infof("PowerShell завершился с кодом: %d", exitErr.ExitCode())
		} else {
			logger.Errorf("Ошибка PowerShell: %v", err)
		}
		return "", "", false, nil
	}

	// Разбираем вывод
	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		logger.Info("Пользователь отменил ввод")
		return "", "", false, nil
	}

	lines := strings.Split(outputStr, "\r\n")
	if len(lines) < 2 {
		lines = strings.Split(outputStr, "\n")
	}

	if len(lines) >= 2 {
		username = strings.TrimSpace(lines[0])
		passwordBase64 := strings.TrimSpace(lines[1])

		// Декодируем пароль из base64
		decoded, err := base64.StdEncoding.DecodeString(passwordBase64)
		if err != nil {
			logger.Errorf("Ошибка декодирования пароля: %v", err)
			return "", "", false, err
		}
		password = string(decoded)

		// Убираем экранирование
		password = strings.ReplaceAll(password, "\\'", "'")
		password = strings.ReplaceAll(password, "\\\"", "\"")
		password = strings.ReplaceAll(password, "\\\\", "\\")

		ok = username != "" && password != ""

		if ok {
			logger.Infof("Учетные данные получены для пользователя: %s", username)
		}

		return username, password, ok, nil
	}

	return "", "", false, nil
}
