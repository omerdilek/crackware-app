package main

import (
	"encoding/json"
	"image/color"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type Download struct {
	Title      string   `json:"title"`
	URIs       []string `json:"uris"`
	UploadDate string   `json:"uploadDate"`
	FileSize   string   `json:"fileSize"`
}

type JSONData struct {
	Name      string     `json:"name"`
	Downloads []Download `json:"downloads"`
}

type DownloadItem struct {
	AppName  string
	Download Download
}

type DiscoverPage struct {
	allData []JSONData // Stores all parsed JSON data

	allItems []DownloadItem // Stores all individual download items from allData

	filteredData []DownloadItem // Stores items after search and filter

	list *widget.List // Fyne list widget to display items

	searchEntry *widget.Entry // Search input field

	sortSelect *widget.Select // Dropdown for sorting options

	multiplayerCheck *widget.Check // New checkbox for multiplayer filter

	statusLabel *widget.Label // Label to display current filter/total status

	window fyne.Window // Reference to the main window
}

// page_discover creates and returns the Discover page content.
func page_discover() fyne.CanvasObject {
	dp := &DiscoverPage{
		window: fyne.CurrentApp().Driver().AllWindows()[0], // Get the main window
	}

	// Multiplayer Checkbox
	dp.multiplayerCheck = widget.NewCheck("Çok Oyunculu Destekli", func(checked bool) {
		dp.loadJSONFiles() // Reload JSON files based on checkbox state
		dp.processData()   // Reprocess the loaded data
		dp.filterAndSort() // Apply current filters and sort
	})

	// Search entry
	dp.searchEntry = widget.NewEntry()
	dp.searchEntry.SetPlaceHolder("🔍 Başlık ara...")
	dp.searchEntry.OnChanged = func(content string) {
		dp.filterAndSort()
	}

	// Sorting options
	dp.sortSelect = widget.NewSelect(
		[]string{"A-Z", "Z-A", "Tarih (Yeni)", "Tarih (Eski)", "Boyut (Büyük)", "Boyut (Küçük)"},
		func(value string) {
			dp.filterAndSort()
		},
	)
	dp.sortSelect.SetSelected("A-Z") // Set initial sort order

	// Status label
	dp.statusLabel = widget.NewLabel("")

	// Initial data load and processing
	dp.loadJSONFiles()
	dp.processData()
	dp.filteredData = dp.allItems
	dp.updateStatusLabel()

	// List widget
	dp.list = widget.NewList(
		func() int {
			return len(dp.filteredData)
		},
		func() fyne.CanvasObject {
			titleLabel := widget.NewLabel("")
			titleLabel.TextStyle.Bold = true

			appLabel := widget.NewLabel("")
			appLabel.TextStyle.Italic = true

			sizeLabel := widget.NewLabel("")
			dateLabel := widget.NewLabel("")

			downloadBtn := widget.NewButtonWithIcon("İndir", theme.DownloadIcon(), nil)

			return container.NewVBox(
				container.NewHBox(widget.NewIcon(theme.DocumentIcon()), titleLabel),
				appLabel,
				container.NewHBox(
					widget.NewIcon(theme.StorageIcon()), sizeLabel,
					widget.NewLabel(" | "),
					widget.NewIcon(theme.HistoryIcon()), dateLabel,
				),
				downloadBtn,
				widget.NewSeparator(),
			)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			if i >= len(dp.filteredData) {
				return
			}
			item := dp.filteredData[i]
			vbox := o.(*fyne.Container)

			titleRow := vbox.Objects[0].(*fyne.Container)
			titleLabel := titleRow.Objects[1].(*widget.Label)
			appLabel := vbox.Objects[1].(*widget.Label)
			infoRow := vbox.Objects[2].(*fyne.Container)
			sizeLabel := infoRow.Objects[1].(*widget.Label)
			dateLabel := infoRow.Objects[4].(*widget.Label)
			downloadBtn := vbox.Objects[3].(*widget.Button)

			titleLabel.SetText(item.Download.Title)
			appLabel.SetText("📦 " + item.AppName)
			sizeLabel.SetText(item.Download.FileSize)

			if parsedDate, err := time.Parse("2006-01-02", item.Download.UploadDate); err == nil {
				dateLabel.SetText(parsedDate.Format("02.01.2006"))
			} else {
				dateLabel.SetText(item.Download.UploadDate) // Fallback to original if parsing fails
			}

			downloadBtn.OnTapped = func() {
				dp.showDownloadDialog(item)
			}
		},
	)

	// Refresh button
	refreshBtn := widget.NewButtonWithIcon("Yenile", theme.ViewRefreshIcon(), func() {
		dp.loadJSONFiles()
		dp.processData()
		dp.filterAndSort()
	})

	// Controls layout
	controls := container.NewBorder(
		nil, nil, nil,
		refreshBtn,
		container.NewVBox(
			dp.searchEntry,
			container.NewHBox(
				widget.NewLabel("Sıralama:"),
				dp.sortSelect,
				widget.NewLabel(" | "),
				dp.statusLabel,
			),
			dp.multiplayerCheck, // Add the new checkbox here
		),
	)

	// Main content layout
	content := container.NewBorder(
		controls,
		nil, nil, nil,
		container.NewScroll(dp.list),
	)
	return content
}

// loadJSONFiles reads JSON data from the sources directory.
// It loads either all JSON files or only onlinefix.json based on the multiplayerCheck state.
func (dp *DiscoverPage) loadJSONFiles() {
	dp.allData = []JSONData{} // Clear previous data

	sourcesDir := "sources"
	if _, err := os.Stat(sourcesDir); os.IsNotExist(err) {
		fmt.Printf("Error: '%s' directory not found.\n", sourcesDir)
		return
	}

	// If multiplayerCheck is checked, only load onlinefix.json
	if dp.multiplayerCheck.Checked {
		onlineFixPath := filepath.Join(sourcesDir, "onlinefix.json")
		if _, err := os.Stat(onlineFixPath); os.IsNotExist(err) {
			fmt.Printf("Warning: 'onlinefix.json' not found at %s. No multiplayer data loaded.\n", onlineFixPath)
			return
		}

		data, err := ioutil.ReadFile(onlineFixPath)
		if err != nil {
			fmt.Printf("Error reading file (%s): %v\n", onlineFixPath, err)
			return
		}

		var jsonData JSONData
		if err := json.Unmarshal(data, &jsonData); err != nil {
			fmt.Printf("JSON parse error (%s): %v\n", onlineFixPath, err)
			return
		}
		if jsonData.Name == "" {
			jsonData.Name = strings.TrimSuffix(filepath.Base(onlineFixPath), ".json")
		}
		dp.allData = append(dp.allData, jsonData)
		return // Exit after loading only onlinefix.json
	}

	// If multiplayerCheck is NOT checked, load all JSON files
	err := filepath.Walk(sourcesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil // Skip directories
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".json") {
			return nil // Skip non-JSON files
		}

		data, err := ioutil.ReadFile(path)
		if err != nil {
			fmt.Printf("Error reading file (%s): %v\n", path, err)
			return nil // Continue to next file on error
		}

		var jsonData JSONData
		if err := json.Unmarshal(data, &jsonData); err != nil {
			fmt.Printf("JSON parse error (%s): %v\n", path, err)
			return nil // Continue to next file on error
		}

		if jsonData.Name == "" {
			jsonData.Name = strings.TrimSuffix(info.Name(), ".json")
		}
		dp.allData = append(dp.allData, jsonData)
		return nil
	})

	if err != nil {
		fmt.Printf("Directory traversal error: %v\n", err)
	}
}

// processData flattens the loaded JSONData into a single list of DownloadItem.
func (dp *DiscoverPage) processData() {
	dp.allItems = []DownloadItem{} // Clear previous items
	for _, data := range dp.allData {
		for _, download := range data.Downloads {
			dp.allItems = append(dp.allItems, DownloadItem{
				AppName:  data.Name,
				Download: download,
			})
		}
	}
}

// filterAndSort applies the search filter and chosen sort order to the data.
func (dp *DiscoverPage) filterAndSort() {
	if dp.statusLabel == nil || dp.list == nil {
		return // Ensure widgets are initialized
	}

	searchText := strings.ToLower(dp.searchEntry.Text)
	if searchText == "" {
		dp.filteredData = dp.allItems
	} else {
		dp.filteredData = []DownloadItem{}
		for _, item := range dp.allItems {
			if strings.Contains(strings.ToLower(item.Download.Title), searchText) ||
				strings.Contains(strings.ToLower(item.AppName), searchText) {
				dp.filteredData = append(dp.filteredData, item)
			}
		}
	}

	// Sorting logic
	sortType := dp.sortSelect.Selected
	sort.Slice(dp.filteredData, func(i, j int) bool {
		itemI := dp.filteredData[i]
		itemJ := dp.filteredData[j]

		switch sortType {
		case "A-Z":
			return itemI.Download.Title < itemJ.Download.Title
		case "Z-A":
			return itemI.Download.Title > itemJ.Download.Title
		case "Tarih (Yeni)":
			// Parse dates, handle errors by pushing unparseable dates to end
			dateI, errI := time.Parse("2006-01-02", itemI.Download.UploadDate)
			dateJ, errJ := time.Parse("2006-01-02", itemJ.Download.UploadDate)
			if errI != nil && errJ != nil {
				return false // Both invalid, maintain original order
			}
			if errI != nil {
				return false // I is invalid, J is valid, J comes first (older first)
			}
			if errJ != nil {
				return true // J is invalid, I is valid, I comes first (newer first)
			}
			return dateI.After(dateJ) // Newer dates first
		case "Tarih (Eski)":
			dateI, errI := time.Parse("2006-01-02", itemI.Download.UploadDate)
			dateJ, errJ := time.Parse("2006-01-02", itemJ.Download.UploadDate)
			if errI != nil && errJ != nil {
				return false
			}
			if errI != nil {
				return true
			}
			if errJ != nil {
				return false
			}
			return dateI.Before(dateJ) // Older dates first
		case "Boyut (Büyük)":
			// Simple string comparison for file size, might need more robust parsing for accurate sorting
			return itemI.Download.FileSize > itemJ.Download.FileSize
		case "Boyut (Küçük)":
			return itemI.Download.FileSize < itemJ.Download.FileSize
		default:
			return itemI.Download.Title < itemJ.Download.Title // Default to A-Z
		}
	})

	dp.updateStatusLabel()
	dp.list.Refresh()
}

// updateStatusLabel updates the label showing the number of displayed items.
func (dp *DiscoverPage) updateStatusLabel() {
	total := len(dp.allItems)
	filtered := len(dp.filteredData)
	if filtered == total {
		dp.statusLabel.SetText(fmt.Sprintf("📊 Toplam %d öğe", total))
	} else {
		dp.statusLabel.SetText(fmt.Sprintf("📊 %d/%d öğe gösteriliyor", filtered, total))
	}
}

// showDownloadDialog displays a dialog with download information.
func (dp *DiscoverPage) showDownloadDialog(item DownloadItem) {
	content := container.NewVBox()

	titleLabel := widget.NewLabel(item.Download.Title)
	titleLabel.TextStyle.Bold = true
	titleLabel.Alignment = fyne.TextAlignCenter
	content.Add(titleLabel)
	content.Add(widget.NewSeparator())

	infoGrid := container.NewGridWithColumns(2,
		widget.NewLabel("Uygulama:"), widget.NewLabel(item.AppName),
		widget.NewLabel("Dosya Boyutu:"), widget.NewLabel(item.Download.FileSize),
		widget.NewLabel("Yükleme Tarihi:"), widget.NewLabel(item.Download.UploadDate),
	)
	content.Add(infoGrid)
	content.Add(widget.NewSeparator())

	if len(item.Download.URIs) > 0 {
		content.Add(widget.NewLabel("İndirme Linki:"))
		// Display multiple URIs if available, or just the first one
		for _, uri := range item.Download.URIs {
			content.Add(widget.NewHyperlink(uri, parseURL(uri)))
		}
	} else {
		content.Add(widget.NewLabel("İndirme Linki Bulunamadı."))
	}

	d := dialog.NewCustom("İndir", "Kapat", content, dp.window)
	d.Resize(fyne.NewSize(400, 350)) // Slightly increase size to accommodate new link display
	d.Show()
}

// parseURL is a helper to parse URL strings, returning nil on error.
func parseURL(raw string) *url.URL {
	u, _ := url.Parse(raw)
	return u
}

// page_home creates and returns the Home page content.
func page_home() fyne.CanvasObject {
	return container.NewVBox(
		widget.NewLabelWithStyle("🎮 LiteGame Platformu", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		canvas.NewLine(color.Gray{Y: 128}),

		widget.NewLabel("📝 Bilgilendirme"),
		widget.NewRichTextFromMarkdown(`**Hoş geldiniz!** Bu platform üzerinden en yeni ve popüler oyunları güvenli bir şekilde indirebilirsiniz. Lütfen aşağıdaki notları ve bağlantıları kontrol etmeyi unutmayın.`),

		widget.NewLabel("🔔 Güncelleme Notları"),
		widget.NewRichTextFromMarkdown(`- **v1.2.0**: Yeni filtreleme sistemi eklendi.  
- **v1.1.5**: Keşfet sayfası performans iyileştirmeleri.  
- **v1.1.0**: Favorilere ekleme özelliği.`),

		canvas.NewLine(color.Gray{Y: 180}),

		widget.NewLabel("🚀 Eklemeyi Düşündüğüm Özellikler"),
		widget.NewRichTextFromMarkdown(`
        - Dahili torrent istemcisi: bu sayede artık tek tıkla torrent adreslerini indirmeyi planlıyorum  
        - Tasarımda yenilikler: daha modern bir arayüz tasarlanacak.   
        - Kütüphane sistemi: oyunları kütüphaneye ekleyip oradan yönetmeyi planlıyorum. 
        - Profil sistemi: hesap oluşturma profili düzenleme ve arkadaş ekleme sistemi.
        `),

		canvas.NewLine(color.Gray{Y: 180}),

		widget.NewLabel("📫 Yardım ve Destek"),
		widget.NewRichTextFromMarkdown("Bir sorunla mı karşılaştınız? Lütfen [discord adresimizi](https://discord.gg/WctNUEWQEj) ziyaret edin ve sorunu bildirin."),

		widget.NewLabel("🔗 Önemli Bağlantılar"),
		container.NewHBox(
			widget.NewHyperlink("GitHub", parseURL("https://github.com/omerdilek/crackware-app")),
			widget.NewHyperlink("Topluluk", parseURL("https://discord.gg/WctNUEWQEj")),
		),
	)
}

// main entry point for the application.
func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("CrackWare App")
	myWindow.Resize(fyne.NewSize(900, 700))

	discoverContent := page_discover()

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Anasayfa", theme.HomeIcon(), page_home()),
		container.NewTabItemWithIcon("Keşfet", theme.SearchIcon(), discoverContent),
	)
	tabs.SetTabLocation(container.TabLocationLeading)

	myWindow.SetContent(tabs)
	myWindow.ShowAndRun()
}
