//go:build esp32

// Example: template round trip (LoadChar, UploadTemplateAuto,
// DownloadTemplateAuto, Store, Delete).
package main

import (
	"time"

	"m5stick-obd2-cnx/examples/common"
	"m5stick-obd2-cnx/lib/finger2"
)

func main() {
	time.Sleep(2 * time.Second)
	dev, err := common.Open(10 * time.Second)
	if err != nil {
		println("open:", err.Error())
		return
	}
	if dev.Templates == 0 {
		println("no templates: enroll one first")
		return
	}
	page, err := dev.FirstFreePage()
	if err != nil {
		println("free page:", err.Error())
		return
	}

	if c, err := dev.LoadChar(1, 0); err != nil || c != finger2.ConfirmOK {
		println("loadchar:", finger2.Code(c).Error())
		return
	}
	tpl, err := dev.UploadTemplateAuto(finger2.TemplateBytes)
	if err != nil {
		println("upload:", err.Error())
		return
	}
	if err := dev.DownloadTemplateAuto(tpl); err != nil {
		println("download:", err.Error())
		return
	}
	if c, err := dev.Store(1, page); err != nil || c != finger2.ConfirmOK {
		println("store:", finger2.Code(c).Error())
		return
	}

	before := dev.Templates
	dev.RefreshTemplates()
	after := dev.Templates
	if c, err := dev.Delete(page, 1); err != nil || c != finger2.ConfirmOK {
		println("delete:", finger2.Code(c).Error())
		return
	}
	dev.RefreshTemplates()
	println("template bytes", len(tpl), "page", page, "templates", before, "->", after,
		"after delete", dev.Templates)
}
