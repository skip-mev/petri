package monitoring

import (
	"context"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"time"
)

// SnapshotGrafanaDashboard takes a snapshot of a grafana dashboard and returns the snapshot URL. This function uses
// the rod library to control a headless browser.
func SnapshotGrafanaDashboard(ctx context.Context, uid, grafanaURL string) (string, error) {
	url, err := launcher.New().Headless(false).Launch()

	if err != nil {
		return "", err
	}

	browser := rod.New().ControlURL(url)

	err = browser.Connect()

	if err != nil {
		return "", err
	}

	defer browser.MustClose()

	loginPage, err := browser.Page(proto.TargetCreateTarget{URL: grafanaURL + "/login"})

	if err != nil {
		return "", err
	}

	err = loginPage.WaitLoad()

	if err != nil {
		return "", err
	}

	usernameInput, err := loginPage.Element("input[name='user']")

	if err != nil {
		return "", err
	}

	err = usernameInput.Input("admin")

	if err != nil {
		return "", err
	}

	passwordInput, err := loginPage.Element("input[name='password']")

	if err != nil {
		return "", err
	}

	err = passwordInput.Input("admin")

	if err != nil {
		return "", err
	}

	submitButton, err := loginPage.Element("button[type='submit']")

	if err != nil {
		return "", err
	}

	err = submitButton.Click(proto.InputMouseButtonLeft, 1)

	if err != nil {
		return "", err
	}

	time.Sleep(time.Second)

	page, err := browser.Page(proto.TargetCreateTarget{URL: grafanaURL + "/d/" + uid})

	if err != nil {
		return "", err
	}

	err = page.WaitLoad()

	if err != nil {
		return "", err
	}

<<<<<<< HEAD
	shareDashboardButton, err := page.Element("[data-testid='data-testid share-button']")

=======
	err = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:             1920 * 5,
		Height:            1080 * 5,
		DeviceScaleFactor: 1,
	})
	if err != nil {
		return "", err
	}

	// The page takes a long time to render, and if you create the snapshot before the data loads it will be empty.
	time.Sleep(time.Second * 10)

	shareDashboardButton, err := page.Element("[data-testid='data-testid Share dashboard']")
>>>>>>> fb0f65b (Perf test changes)
	if err != nil {
		return "", err
	}

	err = shareDashboardButton.Click(proto.InputMouseButtonLeft, 1)

	if err != nil {
		return "", err
	}

<<<<<<< HEAD
	snapshotButton, err := page.Element("[aria-label='Tab Snapshot']")

=======
	snapshotButton, err := page.Element("[data-testid='data-testid Tab Snapshot']")
>>>>>>> fb0f65b (Perf test changes)
	if err != nil {
		return "", err
	}

	err = snapshotButton.Click(proto.InputMouseButtonLeft, 1)

	if err != nil {
		return "", err
	}

	publishButton, err := page.ElementR("span", "Publish to snapshots.raintank.io")

	if err != nil {
		return "", err
	}

	err = publishButton.Click(proto.InputMouseButtonLeft, 1)

	if err != nil {
		return "", err
	}

	// wait for the snapshot to be created

	err = page.WaitElementsMoreThan("input[id='snapshot-url-input']", 0)

	if err != nil {
		return "", err
	}

	snapshotURLInput, err := page.Element("input[id='snapshot-url-input']")

	if err != nil {
		return "", err
	}

	snapshotURL, err := snapshotURLInput.Attribute("value")

	if err != nil {
		return "", err
	}

	return *snapshotURL, err
}
