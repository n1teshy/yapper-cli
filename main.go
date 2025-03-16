package main

import (
	"fmt"

	"github.com/n1teshy/yapper-cli/piper"
)

func main() {
	err := piper.InstallPiper()
	if err != nil {
		fmt.Println(err)
		return
	}
	piper.DownloadVoice("en_US", "amy", "medium")
	// err := piper.DownloadVoiceMaps(false)
	// if err != nil {
	// 	fmt.Println(err)
	// }
	err = piper.TextToWave("hello world", "test.wav")
	if err != nil {
		fmt.Println(err)
	}
}
