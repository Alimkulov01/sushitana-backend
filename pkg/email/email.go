package email

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"strings"
)

// Todo: place it in gateway folder
// Email stores Mail and SMTPServer structs for convenience
type Email struct {
	Mail       _Mail
	SMTPServer _SMTPServer
}

// Mail stores information for sending email
type _Mail struct {
	Sender _Sender

	// To can be given multiple targets for emailing
	To []string

	Subject     string
	Body        string
	MessageBody string
	Auth        smtp.Auth
	tlsconfig   *tls.Config
}

// SenderEmail stores login and password of email of sender
type _Sender struct {
	Login    string
	Password string
}

// SMTPServer stores host and port of server
type _SMTPServer struct {
	Host string
	Port string
}

// ServerName returns concatenated host and port
func (s *_SMTPServer) ServerName() string {
	return s.Host + ":" + s.Port
}

// BuildMessage prepares message for sending
func (mail *_Mail) BuildMessage() string {
	message := ""
	message += fmt.Sprintf("From: %s\r\n", mail.Sender.Login)
	if len(mail.To) > 0 {
		message += fmt.Sprintf("To: %s\r\n", strings.Join(mail.To, ";"))
	}

	message += fmt.Sprintf("Subject: %s\r\n", mail.Subject)
	message += "\r\n" + mail.Body

	return message
}

// SendEmail sends email to adresses given in Email.Mail.to (slice of strings)
// Need to give:
//
// The account from which we send email:
//
// # Email.Mail.Sender.Login
//
// # Email.Mail.Sender.Password
//
// Email.SMTPServer.Host (example: "smtp.yandex.ru")
//
// Email.SMTPServer.Port (example: "465" (for google and yandex))
//
// # Email.Mail.Subject
//
// Email.Mail.Body
func (e *Email) SendEmail() (err error) {

	// build an auth
	e.Mail.Auth = smtp.PlainAuth("", e.Mail.Sender.Login, e.Mail.Sender.Password, e.SMTPServer.Host)

	e.Mail.tlsconfig = &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         e.SMTPServer.Host,
	}

	e.Mail.MessageBody = e.Mail.BuildMessage()

	conn, err := tls.Dial("tcp", e.SMTPServer.ServerName(), e.Mail.tlsconfig)
	if err != nil {
		log.Println("tls.Dial err:", err)
		return
	}

	client, err := smtp.NewClient(conn, e.SMTPServer.Host)
	if err != nil {
		log.Println("smtp.NewClient err:", err)
		return
	}

	// step 1: Use Auth
	if err = client.Auth(e.Mail.Auth); err != nil {
		log.Println("client.Auth err:", err)
		return
	}

	// step 2: add all from and to
	if err = client.Mail(e.Mail.Sender.Login); err != nil {
		log.Println("client.Mail err:", err)
		return
	}
	for _, k := range e.Mail.To {
		if err = client.Rcpt(k); err != nil {
			log.Printf("Sending mail for %s; client.Rcpt err: %v", k, err)
			return
		}
		log.Println(k)
	}

	// Data
	w, err := client.Data()
	if err != nil {
		log.Println("client.Data err:", err)
		return
	}

	_, err = w.Write([]byte(e.Mail.MessageBody))
	if err != nil {
		log.Println("w.Write err:", err)
		return
	}

	err = w.Close()
	if err != nil {
		log.Println("w.Close err:", err)
		return
	}

	err = client.Quit()
	if err != nil {
		log.Println("err while quiting ", err)
		return err
	}

	log.Println("Mail sent successfully")

	return
}
