package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"genealogymapper/internal/domain"
	"genealogymapper/internal/genealogy"
	"genealogymapper/internal/graph"
	"genealogymapper/internal/layout"

	"github.com/lxn/walk"
	"github.com/lxn/win"
)

type application struct {
	service       *genealogy.Service
	window        *walk.MainWindow
	canvas        *graphCanvas
	search        *walk.LineEdit
	peopleList    *walk.ListBox
	peopleModel   *personListModel
	selectedLabel *walk.Label
	selectedInfo  *walk.Label
	status        *walk.Label
	editButton    *walk.PushButton
	addParent     *walk.PushButton
	addChild      *walk.PushButton
	selectedID    string
	showFull      bool
	ctx           context.Context
}

func Run(service *genealogy.Service) error {
	runtime.LockOSThread()
	app := &application{service: service, ctx: context.Background()}
	if err := app.build(); err != nil {
		return err
	}
	app.refreshPeople()
	app.window.SetVisible(true)
	app.window.Run()
	return nil
}

func (a *application) build() error {
	window, err := walk.NewMainWindow()
	if err != nil {
		return err
	}
	a.window = window
	if err := window.SetTitle("Genealogy Mapper"); err != nil {
		return err
	}
	if err := window.SetClientSizePixels(walk.Size{Width: 1280, Height: 760}); err != nil {
		return err
	}
	if err := window.SetLayout(walk.NewVBoxLayout()); err != nil {
		return err
	}

	if err := a.buildToolbar(); err != nil {
		return err
	}
	if err := a.buildBody(); err != nil {
		return err
	}
	status, err := walk.NewLabel(window)
	if err != nil {
		return err
	}
	_ = status.SetText("Connect to Neo4j to begin")
	a.status = status
	return nil
}

func (a *application) buildToolbar() error {
	toolbar, err := walk.NewComposite(a.window)
	if err != nil {
		return err
	}
	if err := toolbar.SetLayout(walk.NewHBoxLayout()); err != nil {
		return err
	}

	newPerson, err := walk.NewPushButton(toolbar)
	if err != nil {
		return err
	}
	if err := newPerson.SetText("New Person"); err != nil {
		return err
	}
	newPerson.Clicked().Attach(func() { a.createPerson() })

	editButton, err := walk.NewPushButton(toolbar)
	if err != nil {
		return err
	}
	if err := editButton.SetText("Edit Person"); err != nil {
		return err
	}
	editButton.SetEnabled(false)
	editButton.Clicked().Attach(func() { a.editPerson() })
	a.editButton = editButton

	addParent, err := walk.NewPushButton(toolbar)
	if err != nil {
		return err
	}
	if err := addParent.SetText("Add Parent"); err != nil {
		return err
	}
	addParent.SetEnabled(false)
	addParent.Clicked().Attach(func() { a.addRelation(true) })
	a.addParent = addParent

	addChild, err := walk.NewPushButton(toolbar)
	if err != nil {
		return err
	}
	if err := addChild.SetText("Add Child"); err != nil {
		return err
	}
	addChild.SetEnabled(false)
	addChild.Clicked().Attach(func() { a.addRelation(false) })
	a.addChild = addChild

	refresh, err := walk.NewPushButton(toolbar)
	if err != nil {
		return err
	}
	if err := refresh.SetText("Refresh"); err != nil {
		return err
	}
	refresh.Clicked().Attach(func() {
		a.refreshPeople()
		a.refreshSelected()
	})

	fit, err := walk.NewPushButton(toolbar)
	if err != nil {
		return err
	}
	if err := fit.SetText("Fit Graph"); err != nil {
		return err
	}
	fit.Clicked().Attach(func() { a.canvas.fit() })

	full, err := walk.NewCheckBox(toolbar)
	if err != nil {
		return err
	}
	if err := full.SetText("Show Full Genealogy"); err != nil {
		return err
	}
	full.CheckedChanged().Attach(func() {
		a.showFull = full.Checked()
		a.refreshSelected()
	})

	importJSON, err := walk.NewPushButton(toolbar)
	if err != nil {
		return err
	}
	if err := importJSON.SetText("Import JSON"); err != nil {
		return err
	}
	importJSON.Clicked().Attach(func() { a.importJSON() })

	exportJSON, err := walk.NewPushButton(toolbar)
	if err != nil {
		return err
	}
	if err := exportJSON.SetText("Export JSON"); err != nil {
		return err
	}
	exportJSON.Clicked().Attach(func() { a.exportJSON() })
	return nil
}

func (a *application) buildBody() error {
	body, err := walk.NewComposite(a.window)
	if err != nil {
		return err
	}
	bodyLayout := walk.NewHBoxLayout()
	if err := body.SetLayout(bodyLayout); err != nil {
		return err
	}
	if err := a.window.Layout().(*walk.BoxLayout).SetStretchFactor(body, 1); err != nil {
		return err
	}

	left, err := walk.NewComposite(body)
	if err != nil {
		return err
	}
	if err := left.SetMinMaxSizePixels(walk.Size{Width: 230, Height: 0}, walk.Size{Width: 330, Height: 0}); err != nil {
		return err
	}
	if err := left.SetLayout(walk.NewVBoxLayout()); err != nil {
		return err
	}
	peopleHeading, err := walk.NewLabel(left)
	if err != nil {
		return err
	}
	_ = peopleHeading.SetText("People")
	search, err := walk.NewLineEdit(left)
	if err != nil {
		return err
	}
	_ = search.SetCueBanner("Search by name")
	a.search = search
	search.TextChanged().Attach(func() { a.refreshPeople() })

	model := &personListModel{}
	peopleList, err := walk.NewListBox(left)
	if err != nil {
		return err
	}
	if err := peopleList.SetModel(model); err != nil {
		return err
	}
	peopleList.CurrentIndexChanged().Attach(func() {
		index := peopleList.CurrentIndex()
		if index >= 0 && index < len(model.people) {
			a.selectPerson(model.people[index].ID)
		}
	})
	left.Layout().(*walk.BoxLayout).SetStretchFactor(peopleList, 1)
	a.peopleList = peopleList
	a.peopleModel = model

	center, err := walk.NewComposite(body)
	if err != nil {
		return err
	}
	if err := center.SetLayout(walk.NewVBoxLayout()); err != nil {
		return err
	}
	canvas, err := newGraphCanvas(center, a.selectPerson)
	if err != nil {
		return err
	}
	center.Layout().(*walk.BoxLayout).SetStretchFactor(canvas.widget, 1)
	a.canvas = canvas
	bodyLayout.SetStretchFactor(center, 1)

	right, err := walk.NewComposite(body)
	if err != nil {
		return err
	}
	if err := right.SetMinMaxSizePixels(walk.Size{Width: 230, Height: 0}, walk.Size{Width: 330, Height: 0}); err != nil {
		return err
	}
	if err := right.SetLayout(walk.NewVBoxLayout()); err != nil {
		return err
	}
	heading, err := walk.NewLabel(right)
	if err != nil {
		return err
	}
	_ = heading.SetText("Selected person")
	selectedLabel, err := walk.NewLabel(right)
	if err != nil {
		return err
	}
	_ = selectedLabel.SetText("No person selected")
	a.selectedLabel = selectedLabel
	selectedInfo, err := walk.NewLabel(right)
	if err != nil {
		return err
	}
	_ = selectedInfo.SetText("Select a node to inspect it.")
	a.selectedInfo = selectedInfo
	return nil
}

func (a *application) createPerson() {
	input, accepted := showPersonDialog(a.window, "New Person", nil)
	if !accepted {
		return
	}
	person, err := a.service.CreatePerson(a.ctx, input)
	if err != nil {
		a.showError(err)
		return
	}
	a.selectedID = person.ID
	a.setStatus(fmt.Sprintf("Created %s", person.Name))
	a.refreshPeople()
	a.refreshSelected()
}

func (a *application) editPerson() {
	if a.selectedID == "" {
		return
	}
	person, err := a.service.GetPerson(a.ctx, a.selectedID)
	if err != nil {
		a.showError(err)
		return
	}
	input, accepted := showPersonDialog(a.window, "Edit Person", &person)
	if !accepted {
		return
	}
	updated, err := a.service.UpdatePerson(a.ctx, a.selectedID, input)
	if err != nil {
		a.showError(err)
		return
	}
	a.setStatus(fmt.Sprintf("Updated %s", updated.Name))
	a.refreshPeople()
	a.refreshSelected()
}

func (a *application) addRelation(addParent bool) {
	if a.selectedID == "" {
		return
	}
	people, err := a.service.SearchPeople(a.ctx, "")
	if err != nil {
		a.showError(err)
		return
	}
	choices := make([]domain.Person, 0, len(people))
	for _, person := range people {
		if person.ID != a.selectedID {
			choices = append(choices, person)
		}
	}
	choice, accepted := showRelationDialog(a.window, choices, addParent)
	if !accepted {
		return
	}

	var created domain.Person
	if choice.newPerson != nil {
		if addParent {
			created, err = a.service.CreatePersonWithChild(a.ctx, *choice.newPerson, a.selectedID)
		} else {
			created, err = a.service.CreatePersonWithParent(a.ctx, *choice.newPerson, a.selectedID)
		}
		a.selectedID = created.ID
	} else if choice.personID != "" {
		if addParent {
			err = a.service.AddParent(a.ctx, a.selectedID, choice.personID)
		} else {
			err = a.service.AddChild(a.ctx, a.selectedID, choice.personID)
		}
	}
	if err != nil {
		a.showError(err)
		return
	}
	if choice.newPerson != nil {
		a.selectedID = created.ID
	}
	a.setStatus("Relationship saved")
	a.refreshPeople()
	a.refreshSelected()
}

func (a *application) importJSON() {
	dialog := walk.FileDialog{
		Title:  "Import Genealogy JSON",
		Filter: "JSON Files (*.json)|*.json|All Files (*.*)|*.*",
	}
	accepted, err := dialog.ShowOpen(a.window)
	if err != nil {
		a.showError(err)
		return
	}
	if !accepted {
		return
	}

	data, err := os.ReadFile(dialog.FilePath)
	if err != nil {
		a.showError(fmt.Errorf("read JSON file: %w", err))
		return
	}
	if walk.MsgBox(a.window, "Genealogy Mapper", "Importing this file will replace the current genealogy. Continue?", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != win.IDYES {
		return
	}
	if err := a.service.ImportJSON(a.ctx, data); err != nil {
		a.showError(err)
		return
	}
	a.selectedID = ""
	a.setStatus(fmt.Sprintf("Imported %s", filepath.Base(dialog.FilePath)))
	a.refreshPeople()
	a.refreshSelected()
}

func (a *application) exportJSON() {
	dialog := walk.FileDialog{
		Title:       "Export Genealogy JSON",
		Filter:      "JSON Files (*.json)|*.json|All Files (*.*)|*.*",
		FilePath:    "genealogy.json",
		FilterIndex: 1,
	}
	accepted, err := dialog.ShowSave(a.window)
	if err != nil {
		a.showError(err)
		return
	}
	if !accepted {
		return
	}

	path := dialog.FilePath
	if filepath.Ext(path) == "" {
		path += ".json"
	}
	data, err := a.service.ExportJSON(a.ctx)
	if err != nil {
		a.showError(err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		a.showError(fmt.Errorf("write JSON file: %w", err))
		return
	}
	a.setStatus(fmt.Sprintf("Exported %s", filepath.Base(path)))
}

func (a *application) selectPerson(personID string) {
	if strings.TrimSpace(personID) == "" {
		return
	}
	a.selectedID = personID
	a.refreshSelected()
}

func (a *application) refreshPeople() {
	if a.peopleModel == nil {
		return
	}
	people, err := a.service.SearchPeople(a.ctx, a.search.Text())
	if err != nil {
		a.setStatus("Could not load people: " + userError(err))
		return
	}
	a.peopleModel.SetPeople(people)
	for index, person := range people {
		if person.ID == a.selectedID {
			_ = a.peopleList.SetCurrentIndex(index)
			break
		}
	}
}

func (a *application) refreshSelected() {
	if a.showFull {
		snapshot, err := a.service.GetFullGraph(a.ctx)
		if err != nil {
			a.setStatus("Could not load graph: " + userError(err))
			return
		}
		a.canvas.setSnapshot(layout.ComputeFull(snapshot, a.selectedID))
		person := domain.Person{}
		for _, candidate := range snapshot.People {
			if candidate.ID == a.selectedID {
				person = candidate
				break
			}
		}
		a.updateInspector(person)
		a.updateActions()
		return
	}
	if a.selectedID == "" {
		a.canvas.setSnapshot(layout.Compute(domain.GraphSnapshot{}, ""))
		a.updateInspector(domain.Person{})
		a.updateActions()
		return
	}
	snapshot, err := a.service.GetNeighborhood(a.ctx, a.selectedID)
	if err != nil {
		a.setStatus("Could not load graph: " + userError(err))
		return
	}
	a.canvas.setSnapshot(layout.Compute(snapshot, a.selectedID))
	person := domain.Person{}
	for _, candidate := range snapshot.People {
		if candidate.ID == a.selectedID {
			person = candidate
			break
		}
	}
	a.updateInspector(person)
	a.updateActions()
}

func (a *application) updateInspector(person domain.Person) {
	if person.ID == "" {
		_ = a.selectedLabel.SetText("No person selected")
		_ = a.selectedInfo.SetText("Select a node to inspect it.")
		return
	}
	_ = a.selectedLabel.SetText(person.Name)
	_ = a.selectedInfo.SetText(personDetails(person))
}

func (a *application) updateActions() {
	enabled := a.selectedID != ""
	a.editButton.SetEnabled(enabled)
	a.addParent.SetEnabled(enabled)
	a.addChild.SetEnabled(enabled)
}

func (a *application) setStatus(text string) {
	if a.status != nil {
		_ = a.status.SetText(text)
	}
}

func (a *application) showError(err error) {
	message := userError(err)
	a.setStatus(message)
	walk.MsgBox(a.window, "Genealogy Mapper", message, walk.MsgBoxOK)
}

func personDetails(person domain.Person) string {
	if person.ID == "" {
		return "Select a node to inspect it."
	}
	birth := formatDate(person.BirthDate)
	death := formatDate(person.DeathDate)
	return fmt.Sprintf("ID: %s\nBorn: %s\nDied: %s", person.ID, birth, death)
}

func formatDate(value *time.Time) string {
	if value == nil {
		return "Unknown"
	}
	return value.Format("2006-01-02")
}

func userError(err error) string {
	switch {
	case errors.Is(err, domain.ErrNameRequired):
		return "A person's name is required."
	case errors.Is(err, domain.ErrSelfRelationship):
		return "A person cannot be their own parent."
	case errors.Is(err, graph.ErrRelationExists):
		return "That parent-child relationship already exists."
	case errors.Is(err, graph.ErrPersonNotFound):
		return "The selected person could not be found. Refresh and try again."
	case errors.Is(err, graph.ErrPersonExists):
		return "That person already exists."
	default:
		return err.Error()
	}
}

type personListModel struct {
	walk.ListModelBase
	people []domain.Person
}

func (m *personListModel) ItemCount() int {
	return len(m.people)
}

func (m *personListModel) Value(index int) interface{} {
	return m.people[index].Name
}

func (m *personListModel) SetPeople(people []domain.Person) {
	m.people = append(m.people[:0], people...)
	m.PublishItemsReset()
}
