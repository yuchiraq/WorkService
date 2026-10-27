package documentfont

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type TimesFonts struct{ Regular, Bold, Italic []byte }

var timesCache struct {
	sync.Mutex
	dir   string
	fonts TimesFonts
}

// Windows fonts are used locally for document embedding, never redistributed as
// web-font downloads. Linux deployments supply their own licensed installation.
func Times() (TimesFonts, error) {
	dir := strings.TrimSpace(os.Getenv("WORKSERVICE_DOCUMENT_FONT_DIR"))
	if dir == "" && runtime.GOOS == "windows" {
		dir = filepath.Join(os.Getenv("WINDIR"), "Fonts")
	}
	if dir == "" {
		for _, candidate := range []string{"/usr/share/fonts/truetype/msttcorefonts", "/usr/share/fonts/truetype/msttcorefonts/times", "/usr/local/share/fonts/msttcorefonts"} {
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				dir = candidate
				break
			}
		}
	}
	if dir == "" {
		return TimesFonts{}, fmt.Errorf("Times New Roman не установлен на сервере. Установите шрифт и задайте WORKSERVICE_DOCUMENT_FONT_DIR")
	}
	timesCache.Lock()
	defer timesCache.Unlock()
	if timesCache.dir == dir && len(timesCache.fonts.Regular) > 0 {
		return timesCache.fonts, nil
	}
	read := func(names ...string) ([]byte, error) {
		for _, name := range names {
			b, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			if len(b) > 10<<20 {
				return nil, fmt.Errorf("Слишком большой файл шрифта")
			}
			if err = embeddingAllowed(b); err != nil {
				return nil, err
			}
			return b, nil
		}
		return nil, fmt.Errorf("В каталоге шрифтов не найдено начертание Times New Roman")
	}
	var fonts TimesFonts
	var err error
	if fonts.Regular, err = read("times.ttf", "Times_New_Roman.ttf", "Times New Roman.ttf"); err != nil {
		return fonts, err
	}
	if fonts.Bold, err = read("timesbd.ttf", "Times_New_Roman_Bold.ttf", "Times New Roman Bold.ttf"); err != nil {
		return fonts, err
	}
	if fonts.Italic, err = read("timesi.ttf", "Times_New_Roman_Italic.ttf", "Times New Roman Italic.ttf"); err != nil {
		return fonts, err
	}
	timesCache.dir, timesCache.fonts = dir, fonts
	return fonts, nil
}

func embeddingAllowed(b []byte) error {
	if len(b) < 12 || binary.BigEndian.Uint32(b[:4]) != 0x10000 {
		return fmt.Errorf("Некорректный TrueType-шрифт")
	}
	n := int(binary.BigEndian.Uint16(b[4:6]))
	if 12+16*n > len(b) {
		return fmt.Errorf("Повреждён каталог шрифта")
	}
	for i := 0; i < n; i++ {
		table := b[12+i*16 : 28+i*16]
		if string(table[:4]) != "OS/2" {
			continue
		}
		offset := int(binary.BigEndian.Uint32(table[8:12]))
		if offset < 0 || offset > len(b)-10 {
			return fmt.Errorf("Повреждена таблица лицензии шрифта")
		}
		flags := binary.BigEndian.Uint16(b[offset+8 : offset+10])
		if flags&(0x0002|0x0100|0x0200) != 0 {
			return fmt.Errorf("Лицензия этого шрифта не допускает используемое в PDF встраивание подмножества")
		}
		return nil
	}
	return fmt.Errorf("У шрифта отсутствуют сведения о разрешении встраивания")
}
