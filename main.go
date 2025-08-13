package main

import (
	"compress/gzip"
	"context"
	"fmt"
	"log"
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

type DBCredentials struct{
	user string
	host string
	port string
}
func BackupMysql(cred DBCredentials, save_path string){
	args := []string{
		"--all-databases",
		"--single-transaction",
		"--quick",
		"--routines",
		"--events",
		"--host=" + cred.host,
		"--port=" + cred.port,
		"--user=" + cred.user,
		"--ssl",
	}

	cmd:=exec.Command(
		"mariadb-dump",
		args...
	)

	// Open export file
	exportFile, err:=os.Create(save_path)
	if err!=nil{
		log.Fatal(err)
	}

	// Gzip file before write
	gzWriter:=gzip.NewWriter(exportFile)

	defer func(){
		err:=gzWriter.Close()
		if err!=nil{
			log.Fatalf("Could not archive file: %v", err)
		}

		err=exportFile.Close()
		if err!=nil{
			log.Fatalf("Could not close file: %v", err)
		}
	}()

	cmd.Stdout=gzWriter
	cmd.Stderr=os.Stderr

	err=cmd.Run()
	if err!=nil{
		log.Fatalf("Mysql backup failed: %v", err);
	}
}
func BackupPostgres(cred DBCredentials, save_path string){
	args := []string{
		"--no-password", 
		"-h", cred.host,
		"-p", cred.port,
		"-U", cred.user,
	}
	cmd := exec.Command("pg_dumpall", args...)
	
	// Open export file
	exportFile, err:=os.Create(save_path)
	if err!=nil{
		log.Fatal(err)
	}

	// Gzip file before write
	gzWriter:=gzip.NewWriter(exportFile)

	defer func(){
		err:=gzWriter.Close()
		if err!=nil{
			log.Fatalf("Could not archive file: %v", err)
		}

		err=exportFile.Close()
		if err!=nil{
			log.Fatalf("Could not close file: %v", err)
		}
	}()


	cmd.Stdout=gzWriter
	cmd.Stderr=os.Stderr

	err=cmd.Run()
	if err!=nil{
		log.Fatalf("Postgres backup failed: %v", err);
	}
}
func BackupMongoDb(cred DBCredentials, save_path string){
	cmd := exec.Command(
		"mongodump",
		"--host", cred.host,
		"--port", cred.port,
		"--username", cred.user,
		"--password", os.Getenv("MONGO_PASS"),
		"--authenticationDatabase", "admin",
		"--archive=" + save_path,
		"--gzip",
	)
	cmd.Stderr=os.Stderr

	err:=cmd.Run()
	if err!=nil{
		log.Fatalf("Mongo backup failed: %v", err);
	}
}
func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}
func loadSecretFilesIfExist() error{
	mysql_pass_file:=os.Getenv("MYSQL_PASS_FILE")
	if mysql_pass_file!=""{
		data, err:=os.ReadFile(mysql_pass_file)
		if err!=nil{
			return err;
		}
		os.Setenv("MYSQL_PWD", string(data))
		log.Printf("Mysql password loaded from file")
	}else if os.Getenv("MYSQL_PWD")==""{
		log.Fatal("One form of password for MySQL must be set")
	}

	postgres_pass_file:=os.Getenv("POSTGRES_PASS_FILE")
	if postgres_pass_file!=""{
		data, err:=os.ReadFile(postgres_pass_file)
		if err!=nil{
			return err;
		}

		os.Setenv("PGPASSWORD", string(data))
		log.Print("Postgres password loaded from file")
	}else if os.Getenv("PGPASSWORD")==""{
		log.Fatal("One form of password for PostgreSQL must be set")
	}

	mongo_pass_file:=os.Getenv("MONGO_PASS_FILE")
	if mongo_pass_file!=""{
		data, err:=os.ReadFile(mongo_pass_file)
		if err!=nil{
			return err;
		}

		os.Setenv("MONGO_PASS", string(data))
		log.Print("Mongo password loaded from file")
	}else if os.Getenv("MONGO_PASS")==""{
		log.Fatal("One form of password for MongoDB must be set")
	}
	return nil
}
func main(){
	ctx:=context.Background()

	// Load Secrets 
	loadSecretFilesIfExist()

	// Setup s3 client
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
	mysql_cred:=DBCredentials{
		host: os.Getenv("MYSQL_HOST"),
		port: getEnv("MYSQL_PORT", "3306"),
		user: os.Getenv("MYSQL_USER"),
	}
	// If set then do backup
	if mysql_cred.host!="" && mysql_cred.user!=""{
		file:=filepath.Join(exportDir, fmt.Sprintf("./mysql-%s.sql.gz", time.Now().Format("2006-01-02")))
		BackupMysql(mysql_cred, file)
		err = s3Client.UploadFile(ctx, "mysql", file)
		if err!=nil{
			log.Fatalf("MySQL S3 Upload failed: %v", err)
		}else{
			log.Printf("MySQL dump successful")
		}
	}
	

	// Postgres
	postgres_cred:=DBCredentials{
		host: os.Getenv("POSTGRES_HOST"),
		port: getEnv("POSTGRES_PORT", "5432"),
		user: os.Getenv("POSTGRES_USER"),
	}
	// If set then do backup
	if postgres_cred.host!="" && postgres_cred.user!=""{
		file:=filepath.Join(exportDir, fmt.Sprintf("./postgres-%s.sql.gz", time.Now().Format("2006-01-02")))
		BackupPostgres(postgres_cred, file)
		err = s3Client.UploadFile(ctx, "postgres", file)
		if err!=nil{
			log.Fatalf("PostgreSQL S3 Upload failed: %v", err)
		}else{
			log.Printf("PostgreSQL dump successful")
		}
	}

	// Mongo
	mongo_cred:=DBCredentials{
		host: os.Getenv("MONGO_HOST"),
		port: getEnv("MONGO_PORT", "27017"),
		user: os.Getenv("MONGO_USER"),
	}
	// If set then do backup
	if mongo_cred.host!="" && mongo_cred.user!=""{
		file:=filepath.Join(exportDir, fmt.Sprintf("./mongo-%s.archive.gz", time.Now().Format("2006-01-02")))
		BackupMongoDb(mongo_cred, file)
		err = s3Client.UploadFile(ctx, "mongo", file)
		if err!=nil{
			log.Fatalf("MongoDB S3 Upload failed: %v", err)
		}else{
			log.Printf("MongoDB dump successful")
		}
	}
}