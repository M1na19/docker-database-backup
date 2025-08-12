package main

import (
	"compress/gzip"
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)
type S3Client struct {
	uploader *manager.Uploader
	bucket   string
}

func NewS3Client(ctx context.Context) (*S3Client, error) {
	bucket := os.Getenv("S3_BUCKET")
	endpoint := os.Getenv("S3_ENDPOINT")
	forcePathStyle := strings.EqualFold(os.Getenv("S3_FORCE_PATH_STYLE"), "true")

	if bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET is required")
	}
	
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	s3opts := func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		if forcePathStyle {
			o.UsePathStyle = true
		}
	}

	client := s3.NewFromConfig(cfg, s3opts)
	up := manager.NewUploader(client)

	return &S3Client{
		uploader: up,
		bucket:   bucket,
	}, nil
}
func (c *S3Client) UploadFile(ctx context.Context, prefix string, path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()

	key:=filepath.Join(prefix,filepath.Base(path))
    _, err = c.uploader.Upload(ctx, &s3.PutObjectInput{
        Bucket: &c.bucket,
        Key:    &key,
        Body:   f,
    })
    return err
}

func BackupMysql(uri string, save_path string){
	// Parse URI
	u, err := url.Parse(uri)
	if err != nil {
		log.Fatalf("invalid MYSQL_URI: %v", err)
	}

	user := u.User.Username()
	pass, _ := u.User.Password()

	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Host
		port = "3306"
	}
	if user == "" || host == "" {
		log.Fatalf("MYSQL_URI must include user and host")
	}

	args := []string{
		"--all-databases",
		"--single-transaction",
		"--quick",
		"--routines",
		"--events",
		"--host=" + host,
		"--port=" + port,
		"--user=" + user,
	}

	cmd:=exec.Command(
		"mysqldump",
		args...
	)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+pass)

	// Open export file
	exportFile, err:=os.Create(save_path)
	if err!=nil{
		log.Fatal(err)
	}
	defer exportFile.Close()

	// Gzip file before write
	gzWriter:=gzip.NewWriter(exportFile)
	defer gzWriter.Close()

	cmd.Stdout=gzWriter
	cmd.Stderr=os.Stderr

	err=cmd.Run()
	if err!=nil{
		log.Fatalf("Mysql backup failed: %v", err);
	}
}
func BackupPostgres(uri string, save_path string){
	u, err := url.Parse(uri)
	if err != nil {
		log.Fatalf("invalid POSTGRES_URI: %v", err)
	}
	user := u.User.Username()
	pass, _ := u.User.Password()

	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Host
		port = "5432"
	}
	if user == "" || host == "" {
		log.Fatalf("POSTGRES_URI must include user and host")
	}

	args := []string{
		"--no-password", 
		"-h", host,
		"-p", port,
		"-U", user,
	}
	cmd := exec.Command("pg_dumpall", args...)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+pass)
	
	// Open export file
	exportFile, err:=os.Create(save_path)
	if err!=nil{
		log.Fatal(err)
	}
	defer exportFile.Close()

	// Gzip file before write
	gzWriter:=gzip.NewWriter(exportFile)
	defer gzWriter.Close()

	cmd.Stdout=gzWriter
	cmd.Stderr=os.Stderr

	err=cmd.Run()
	if err!=nil{
		log.Fatalf("Postgres backup failed: %v", err);
	}
}
func BackupMongoDb(uri string,save_path string){
	cmd := exec.Command(
		"mongodump",
		"--uri="+uri,
		"--archive="+ save_path,
		"--gzip",
	)
	cmd.Stderr=os.Stderr

	err:=cmd.Run()
	if err!=nil{
		log.Fatalf("Mongo backup failed: %v", err);
	}
}
func main(){
	ctx:=context.Background()

	s3Client,err:=NewS3Client(ctx)
	if err!=nil{
		log.Fatalf("Failed to create S3 client: %v", err)
	}

	exportDir:=os.Getenv("EXPORT_DIR")
	if exportDir==""{
		exportDir="./backups";
	}
	// Ensure export directory exists
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		log.Fatalf("Failed to create export directory %s: %v", exportDir, err)
	}
	

	// MySQL
	mysql_uri:=os.Getenv("MYSQL_URI")
	if mysql_uri==""{
		log.Print("MySQL uri is not set, no backup will be done")
	}else{
		file:=filepath.Join(exportDir, fmt.Sprintf("./mysql-%s.sql.gz", time.Now().Format("2006-01-02")))
		BackupMysql(mysql_uri, file)
		err := s3Client.UploadFile(ctx, "mysql", file)
		if err!=nil{
			log.Fatalf("MySQL S3 Upload failed: %v", err)
		}else{
			log.Printf("MySQL dump successful")
		}
	}

	// Postgres
	postgres_uri:=os.Getenv("POSTGRES_URI")
	if postgres_uri==""{
		log.Print("PostgreSQL uri is not set, no backup will be done")
	}else{
		file:=filepath.Join(exportDir, fmt.Sprintf("./postgres-%s.sql.gz", time.Now().Format("2006-01-02")))
		BackupPostgres(postgres_uri, file)
		err := s3Client.UploadFile(ctx, "postgres", file)
		if err!=nil{
			log.Fatalf("Postgres S3 Upload failed: %v", err)
		}else{
			log.Printf("Postgres dump successful")
		}
	}

	// Mongo
	mongo_uri:=os.Getenv("MONGO_URI")
	if mongo_uri==""{
		log.Print("MongoDB uri is not set, no backup will be done")
	}else{
		file:=filepath.Join(exportDir, fmt.Sprintf("./mongo-%s.archive.gz", time.Now().Format("2006-01-02")))
		BackupMongoDb(mongo_uri, file)
		err := s3Client.UploadFile(ctx, "mongo", file)
		if err!=nil{
			log.Fatalf("MongoDB S3 Upload failed: %v", err)
		}else{
			log.Printf("MongoDB dump successful")
		}
	}
}