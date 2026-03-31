package util

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/gomail.v2"

	"platform/utils"
)

// EmailConfig 邮件配置
type EmailConfig struct {
	SMTPServer   string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	FromEmail    string
	ToEmail      string
}

// ApplicationNotification 商用试用申请通知
type ApplicationNotification struct {
	Username    string
	Email       string
	Phone       string
	Company     string
	Industry    string
	UserCount   string
	Description string
}

var emailConfigLoaded bool
var cachedEmailConfig EmailConfig

// LoadEmailConfig 加载邮件配置
func LoadEmailConfig() EmailConfig {
	if emailConfigLoaded {
		return cachedEmailConfig
	}

	// 加载 .env 文件中的环境变量（从 backend/ 目录往上一级找到项目根目录的 .env）
	godotenv.Load("../.env")

	smtpServer := utils.GetEnv("SMTP_SERVER", "smtp.gmail.com")
	smtpPort := utils.GetEnv("SMTP_PORT", "587")
	smtpUsername := utils.GetEnv("SMTP_USERNAME", "")
	smtpPassword := utils.GetEnv("SMTP_PASSWORD", "")
	fromEmail := utils.GetEnv("FROM_EMAIL", "")
	toEmail := utils.GetEnv("TO_EMAIL", "")

	cachedEmailConfig = EmailConfig{
		SMTPServer:   smtpServer,
		SMTPPort:     smtpPort,
		SMTPUsername: smtpUsername,
		SMTPPassword: smtpPassword,
		FromEmail:    fromEmail,
		ToEmail:      toEmail,
	}
	emailConfigLoaded = true

	return cachedEmailConfig
}

// SendEmail 发送邮件

func SendEmail(to, subject, body string) error {
	config := LoadEmailConfig()

	// 如果没有配置邮件，直接返回成功（开发环境不发送邮件）
	if config.SMTPUsername == "" || config.FromEmail == "" {
		fmt.Printf("[邮件通知] 开发环境未配置邮件，跳过发送邮件\n")
		return nil
	}

	// 如果没有指定接收者，使用配置的默认接收者
	if to == "" {
		to = config.ToEmail
	}

	if to == "" {
		return fmt.Errorf("未配置邮件接收者")
	}

	// 使用 gomail 发送
	err := sendWithGomail(config.SMTPServer, config.SMTPPort, config.SMTPUsername, config.SMTPPassword, config.FromEmail, to, subject, body)
	if err != nil {
		fmt.Printf("[ERROR] 发送邮件失败: %v\n", err)
		return err
	}

	return nil
}

// 使用 gomail 发送邮件
func sendWithGomail(server, port, username, password, from, to, subject, body string) error {

	m := gomail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	// 端口转换为整数
	var portInt int
	fmt.Sscanf(port, "%d", &portInt)

	d := gomail.NewDialer(server, portInt, username, password)

	// 只有端口 465 使用隐式 SSL
	if port == "465" {
		d.SSL = true
		d.TLSConfig = &tls.Config{
			ServerName:         server,
			InsecureSkipVerify: true,
		}
	} else {
		// 端口 587 使用 STARTTLS
		d.SSL = false
		d.TLSConfig = &tls.Config{
			ServerName:         server,
			InsecureSkipVerify: true,
		}
	}

	return d.DialAndSend(m)
}

// SendApplicationNotification 发送商用试用申请通知邮件
func SendApplicationNotification(app ApplicationNotification) error {
	subject := "【商用试用申请】代码审计平台 - " + app.Company

	currentTime := time.Now().Format("2006-01-02 15:04:05")

	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 700px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 25px; text-align: center; border-radius: 10px 10px 0 0; }
        .header h1 { margin: 0; font-size: 24px; }
        .header p { margin: 5px 0 0 0; opacity: 0.9; font-size: 14px; }
        .content { background: #ffffff; padding: 25px; border: 1px solid #e5e7eb; }
        .section { margin-bottom: 20px; }
        .section-title { background: #f3f4f6; padding: 10px 15px; border-radius: 6px; font-weight: bold; color: #374151; margin-bottom: 10px; }
        .info-table { width: 100%%; border-collapse: collapse; }
        .info-table td { padding: 8px 10px; border-bottom: 1px solid #f3f4f6; }
        .info-table td:first-child { font-weight: bold; color: #6b7280; width: 120px; }
        .info-table td:last-child { color: #111827; }
        .description-box { background: #f9fafb; padding: 15px; border-radius: 6px; border-left: 4px solid #667eea; }
        .urgent { color: #dc2626; font-weight: bold; }
        .footer { background: #1f2937; color: #9ca3af; padding: 15px; text-align: center; border-radius: 0 0 10px 10px; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>📋 商用试用申请通知</h1>
            <p>代码审计平台 - 收到新的企业试用申请</p>
        </div>
        <div class="content">
            <div class="section">
                <div class="section-title">👤 申请人基本信息</div>
                <table class="info-table">
                    <tr><td>申请账号</td><td>%s</td></tr>
                    <tr><td>联系邮箱</td><td>%s</td></tr>
                    <tr><td>联系电话</td><td>%s</td></tr>
                </table>
            </div>
            
            <div class="section">
                <div class="section-title">🏢 企业信息</div>
                <table class="info-table">
                    <tr><td>企业名称</td><td>%s</td></tr>
                    <tr><td>所属行业</td><td>%s</td></tr>
                    <tr><td>预计用户数</td><td>%s</td></tr>
                </table>
            </div>
            
            <div class="section">
                <div class="section-title">💬 申请说明</div>
                <div class="description-box">%s</div>
            </div>
            
            <div class="section">
                <div class="section-title">📅 申请时间</div>
                <table class="info-table">
                    <tr><td>提交时间</td><td>%s</td></tr>
                </table>
            </div>
        </div>
        <div class="footer">
            <p>此邮件由代码审计平台自动发送，请及时处理</p>
        </div>
    </div>
</body>
</html>
	`, app.Username, app.Email, app.Phone, app.Company, app.Industry, app.UserCount, app.Description, currentTime)

	return SendEmail("", subject, body)
}
