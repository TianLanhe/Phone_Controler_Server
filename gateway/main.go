package main

import (
    "net/http"
    "log"
    "os"

    "github.com/grpc-ecosystem/grpc-gateway/runtime"
    "golang.org/x/net/context"
    "google.golang.org/grpc"

    gw "phonecontrol/phonecontrol"
)

const LISTEN_PORT = "8080"
const SERVE_PORT = "16868"

func main(){
    ctx,cancel := context.WithCancel(context.Background())
    defer cancel()

    mux := runtime.NewServeMux()
    opts := []grpc.DialOption{grpc.WithInsecure()}

    err := gw.RegisterServiceHandlerFromEndpoint(ctx,mux,"0.0.0.0:"+SERVE_PORT,opts)
    if err != nil {
        log.Println("error:",err)
        os.Exit(1)
    }

    if err = http.ListenAndServe("0.0.0.0:"+LISTEN_PORT,mux);err!=nil{
        log.Println("error: ",err)
    }
}

