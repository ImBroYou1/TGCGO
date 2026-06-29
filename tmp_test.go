package main
import (
    "fmt"
    "TGCGO/config"
)
func main(){
    config.Init()
    fmt.Println("UsePassword:", config.Load().UsePassword)
    fmt.Println("Hash:", config.Load().PasswordHash)
    fmt.Println("Verify correct:", config.VerifyPassword("testpass"))
    fmt.Println("Verify wrong:", config.VerifyPassword("wrong"))
}
