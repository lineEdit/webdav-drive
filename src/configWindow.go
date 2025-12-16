package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode"
)

// Получаем список свободных букв дисков
func getAvailableDriveLetters() []string {
	var availableLetters []string

	// Получаем список использованных букв
	cmd := exec.Command("wmic", "logicaldisk", "get", "deviceid")
	output, err := cmd.Output()
	if err != nil {
		// В случае ошибки возвращаем стандартный набор
		return []string{"Z:", "Y:", "X:", "W:", "V:", "U:", "T:", "S:", "R:", "Q:", "P:", "O:", "N:", "M:", "L:", "K:", "J:", "I:", "H:", "G:", "F:", "E:", "D:", "C:"}
	}

	// Парсим вывод, чтобы получить использованные буквы
	usedLetters := make(map[string]bool)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 2 && line[1] == ':' && unicode.IsLetter(rune(line[0])) {
			usedLetters[strings.ToUpper(line[:1])] = true
		}
	}

	// Формируем список доступных букв (от Z до C)
	for letter := 'Z'; letter >= 'C'; letter-- {
		letterStr := string(letter) + ":"
		if !usedLetters[string(letter)] {
			availableLetters = append(availableLetters, letterStr)
		}
	}

	// Если нет свободных букв, возвращаем хотя бы Z
	if len(availableLetters) == 0 {
		availableLetters = []string{"Z:"}
	}

	return availableLetters
}

func openConfig() {
	logger.Info("Открытие редактора конфигурации")

	cfg, err := loadConfig()
	if err != nil {
		cfg = &Config{
			DriveLetter: "\"Z:\"",
			WebDAVURL:   "\"\"",
		}
	}

	availableLetters := getAvailableDriveLetters()
	currentDrive := strings.Trim(cfg.DriveLetter, "\"")
	currentURL := strings.Trim(cfg.WebDAVURL, "\"")

	// Формируем строку с буквами для PowerShell
	var lettersPS []string
	for _, letter := range availableLetters {
		lettersPS = append(lettersPS, fmt.Sprintf("'%s'", letter))
	}
	lettersStr := strings.Join(lettersPS, ", ")

	psCommand := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$form = New-Object System.Windows.Forms.Form
$form.Text = 'WebDAV Drive - Настройки'
$form.Size = New-Object System.Drawing.Size(500, 250)
$form.StartPosition = 'CenterScreen'
$form.FormBorderStyle = 'FixedDialog'
$form.MaximizeBox = $false
$form.MinimizeBox = $false

# Буква диска
$driveLabel = New-Object System.Windows.Forms.Label
$driveLabel.Location = New-Object System.Drawing.Point(20, 20)
$driveLabel.Size = New-Object System.Drawing.Size(120, 20)
$driveLabel.Text = 'Буква диска:'

$driveCombo = New-Object System.Windows.Forms.ComboBox
$driveCombo.Location = New-Object System.Drawing.Point(150, 20)
$driveCombo.Size = New-Object System.Drawing.Size(100, 20)
$driveCombo.DropDownStyle = 'DropDownList'

$letters = @(%s)
foreach ($letter in $letters) {
    [void] $driveCombo.Items.Add($letter)
}

if ('%s' -ne '' -and $letters -contains '%s') {
    $driveCombo.Text = '%s'
} else {
    $driveCombo.SelectedIndex = 0
}

# URL
$urlLabel = New-Object System.Windows.Forms.Label
$urlLabel.Location = New-Object System.Drawing.Point(20, 50)
$urlLabel.Size = New-Object System.Drawing.Size(120, 20)
$urlLabel.Text = 'WebDAV URL:'

$urlTextBox = New-Object System.Windows.Forms.TextBox
$urlTextBox.Location = New-Object System.Drawing.Point(150, 50)
$urlTextBox.Size = New-Object System.Drawing.Size(300, 20)
$urlTextBox.Text = '%s'

# Кнопки
$saveButton = New-Object System.Windows.Forms.Button
$saveButton.Location = New-Object System.Drawing.Point(150, 90)
$saveButton.Size = New-Object System.Drawing.Size(75, 30)
$saveButton.Text = 'Сохранить'

$cancelButton = New-Object System.Windows.Forms.Button
$cancelButton.Location = New-Object System.Drawing.Point(235, 90)
$cancelButton.Size = New-Object System.Drawing.Size(75, 30)
$cancelButton.Text = 'Отмена'

# Добавляем элементы
$form.Controls.Add($driveLabel)
$form.Controls.Add($driveCombo)
$form.Controls.Add($urlLabel)
$form.Controls.Add($urlTextBox)
$form.Controls.Add($saveButton)
$form.Controls.Add($cancelButton)

# Результат
$script:result = $null

$saveButton.Add_Click({
    $drive = $driveCombo.Text.Trim()
    $url = $urlTextBox.Text.Trim()
    
    if ($drive -eq '') {
        [System.Windows.Forms.MessageBox]::Show('Выберите букву диска', 'Ошибка', 'OK', 'Error')
        return
    }
    
    if ($url -eq '') {
        [System.Windows.Forms.MessageBox]::Show('Введите URL WebDAV сервера', 'Ошибка', 'OK', 'Error')
        return
    }
    
    $url = $url.TrimEnd('/')
    $script:result = @($drive, $url)
    $form.Close()
})

$cancelButton.Add_Click({
    $form.Close()
})

$form.Add_Shown({ $urlTextBox.Focus(); $urlTextBox.SelectAll() })
$form.ShowDialog() | Out-Null

if ($script:result -ne $null) {
    Write-Output $script:result[0]
    Write-Output $script:result[1]
}
`, lettersStr, currentDrive, currentDrive, currentDrive, currentURL)

	cmd := exec.Command("powershell",
		"-ExecutionPolicy", "Bypass",
		"-NoProfile",
		"-WindowStyle", "Normal",
		"-Command", psCommand)

	output, err := cmd.Output()
	if err != nil {
		logger.Infof("Редактирование отменено: %v", err)
		return
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		logger.Info("Сохранение отменено")
		return
	}

	lines := strings.Split(outputStr, "\n")
	if len(lines) >= 2 {
		drive := strings.TrimSpace(lines[0])
		url := strings.TrimSpace(lines[1])

		// ВАЖНО: НЕ добавляем кавычки, YAML сам их поставит если нужно
		newCfg := Config{
			DriveLetter: drive,
			WebDAVURL:   url,
		}

		data, err := json.Marshal(&newCfg)
		if err != nil {
			logger.Errorf("Ошибка маршалинга конфига: %v", err)
			return
		}

		if err := os.WriteFile(getConfigPath(), data, 0600); err != nil {
			logger.Errorf("Ошибка записи конфига: %v", err)
			return
		}

		// Обновляем глобальную конфигурацию
		globalCfg = &newCfg

		logger.Infof("Конфиг сохранен: %s -> %s", drive, url)
	}
}
