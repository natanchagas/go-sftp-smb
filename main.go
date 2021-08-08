package main

import (
	"fmt"
	"log"
	"net"
	"os"

	smb2 "github.com/hirochachacha/go-smb2"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

/*
Environment variables:
	SFTP: SFTP Endpoint - URL, Server or Address
	SFTP_USERNAME: Username to connect to the SFTP Endpoint
	SFTP_PASSWORD: Password to connect to the SFTP Endpoint
	SMB: SMB Endpoint - Server or Address
	SMB_USERNAME: Username to connect to the SMB Endpoint
	SMB_PASSWORD: Password to connect to the SMB Endpoint
	SMB_DOMAIN: User domain to connect to the SMB Endpoint
	MOUNT: Name of the folder to mount
	PATH: Path inside the mount point to the file. If you're already in the folder, set it as an empty string
*/

func sshConn(user string, pwd string, ftp_endpoint string) (*ssh.Client, error) {

	// Configure SSH Connection
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pwd),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Initialize SSH Connection
	conn, err := ssh.Dial("tcp", ftp_endpoint+":22", config)
	if err != nil {
		log.Fatal("Unable to connect to", ftp_endpoint, ". Error:", err.Error())
	}

	return conn, err
}

func sftpConn(conn *ssh.Client) (*sftp.Client, error) {

	client, err := sftp.NewClient(conn)
	if err != nil {
		log.Fatal("Unable to open SFTP connection. Error:", err.Error())
	}

	return client, err
}

func tcpConn(smb_endpoint string) net.Conn {
	conn, err := net.Dial("tcp", smb_endpoint+":445")
	if err != nil {
		log.Fatal("Unable to connect to SMB port. Error:", err.Error())
	}

	return conn
}

func smbConn(conn net.Conn, user string, password string, domain string) (*smb2.Session, error) {

	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     user,
			Password: password,
			Domain:   domain,
		},
	}

	session, err := d.Dial(conn)
	if err != nil {
		log.Fatal("Unable to open SMB connection. Error:", err.Error())
	}

	return session, err
}

func main() {

	// Open ssh connection
	conn, _ := sshConn(os.Getenv("SFTP_USERNAME"), os.Getenv("SFTP_PASSWORD"), os.Getenv("SFTP"))
	defer conn.close()

	// Open stfp connection using ssh connection
	client, _ := sftpConn(conn)
	defer client.close()

	// Do something on SFTP
	wd, err := client.Getwd()
	if err != nil {
		log.Fatal("User doesn't have permissions to get the current directory. Error:", err.Error())
	}
	fmt.Println("SFTP folder:", wd)

	files, files_err := client.ReadDir(wd)
	if files_err != nil {
		log.Fatal("User doesn't have permissions to list files in the current directory. Error:", err.Error())
	}

	tcpconn := tcpConn(os.Getenv("SMB"))
	defer tcpconn.Close()

	session, _ := smbConn(tcpconn, os.Getenv("SMB_USERNAME"), os.Getenv("SMB_PASSWORD"), os.Getenv("SMB_DOMAIN"))
	defer session.Logoff()

	fs, fs_err := session.Mount(os.Getenv("MOUNT"))
	if fs_err != nil {
		log.Fatal("Unable to mount share. Error:", fs_err.Error())
	}

	for _, file := range files {
		if !file.IsDir() {
			data, data_err := client.Open(wd + "/" + file.Name())
			if data_err != nil {
				log.Fatal("Unable to read file. Error:", data_err.Error())
			}

			dstFile, create_err := fs.Create(os.Getenv("PATH") + file.Name())
			if create_err != nil {
				log.Fatal("Unable to create local file:", file.Name(), "Error:", create_err.Error())
			}
			defer dstFile.Close()

			data.WriteTo(dstFile)

			delete_err := wd.Remove(file.Name())
			if delete_err != nil {
				log.Fatal("Unable to delete file:", file.Name(), "Error:", delete_err.Error())
			}
		}
	}
}
