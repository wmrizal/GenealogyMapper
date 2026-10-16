package ui

import (
	"strings"
	"time"

	"github.com/lxn/walk"
	"genealogymapper/internal/domain"
	"genealogymapper/internal/genealogy"
)

type relationChoice struct {
	personID  string
	newPerson *genealogy.CreatePersonInput
}

func showPersonDialog(owner walk.Form, title string, existing *domain.Person) (genealogy.CreatePersonInput, bool) {
	dialog, err := walk.NewDialogWithFixedSize(owner)
	if err != nil {
		return genealogy.CreatePersonInput{}, false
	}
	if err := dialog.SetTitle(title); err != nil {
		return genealogy.CreatePersonInput{}, false
	}
	if err := dialog.SetClientSizePixels(walk.Size{Width: 420, Height: 250}); err != nil {
		return genealogy.CreatePersonInput{}, false
	}
	if err := dialog.SetLayout(walk.NewVBoxLayout()); err != nil {
		return genealogy.CreatePersonInput{}, false
	}

	nameLabel, _ := walk.NewLabel(dialog)
	_ = nameLabel.SetText("Name")
	name, _ := walk.NewLineEdit(dialog)
	_ = name.SetCueBanner("Required")
	if existing != nil {
		_ = name.SetText(existing.Name)
	}

	birthLabel, _ := walk.NewLabel(dialog)
	_ = birthLabel.SetText("Birth date")
	birthDate, _ := walk.NewDateEditWithNoneOption(dialog)
	_ = birthDate.SetFormat("yyyy-MM-dd")
	if existing != nil && existing.BirthDate != nil {
		_ = birthDate.SetDate(*existing.BirthDate)
	}

	deathLabel, _ := walk.NewLabel(dialog)
	_ = deathLabel.SetText("Death date")
	deathDate, _ := walk.NewDateEditWithNoneOption(dialog)
	_ = deathDate.SetFormat("yyyy-MM-dd")
	if existing != nil && existing.DeathDate != nil {
		_ = deathDate.SetDate(*existing.DeathDate)
	}

	buttons, _ := walk.NewComposite(dialog)
	_ = buttons.SetLayout(walk.NewHBoxLayout())
	okButton, _ := walk.NewPushButton(buttons)
	okText := "Create"
	if existing != nil {
		okText = "Save"
	}
	_ = okButton.SetText(okText)
	cancelButton, _ := walk.NewPushButton(buttons)
	_ = cancelButton.SetText("Cancel")
	_ = dialog.SetDefaultButton(okButton)
	_ = dialog.SetCancelButton(cancelButton)

	var input genealogy.CreatePersonInput
	accepted := false
	okButton.Clicked().Attach(func() {
		if strings.TrimSpace(name.Text()) == "" {
			walk.MsgBox(dialog, "Genealogy Mapper", "A person's name is required.", walk.MsgBoxOK)
			return
		}
		input = genealogy.CreatePersonInput{
			Name:      name.Text(),
			BirthDate: optionalDate(birthDate),
			DeathDate: optionalDate(deathDate),
		}
		accepted = true
		dialog.Accept()
	})
	cancelButton.Clicked().Attach(dialog.Cancel)
	dialog.Run()
	return input, accepted
}

func showRelationDialog(owner walk.Form, people []domain.Person, addParent bool) (relationChoice, bool) {
	title := "Add Child"
	newTitle := "New Child"
	if addParent {
		title = "Add Parent"
		newTitle = "New Parent"
	}
	dialog, err := walk.NewDialogWithFixedSize(owner)
	if err != nil {
		return relationChoice{}, false
	}
	if err := dialog.SetTitle(title); err != nil {
		return relationChoice{}, false
	}
	if err := dialog.SetClientSizePixels(walk.Size{Width: 420, Height: 190}); err != nil {
		return relationChoice{}, false
	}
	if err := dialog.SetLayout(walk.NewVBoxLayout()); err != nil {
		return relationChoice{}, false
	}

	heading, _ := walk.NewLabel(dialog)
	_ = heading.SetText("Link an existing person or create a new one")
	combo, _ := walk.NewDropDownBox(dialog)
	model := &personListModel{}
	model.SetPeople(people)
	_ = combo.SetModel(model)
	if len(people) > 0 {
		_ = combo.SetCurrentIndex(0)
	}

	buttons, _ := walk.NewComposite(dialog)
	_ = buttons.SetLayout(walk.NewHBoxLayout())
	linkButton, _ := walk.NewPushButton(buttons)
	_ = linkButton.SetText("Link Existing")
	linkButton.SetEnabled(len(people) > 0)
	newButton, _ := walk.NewPushButton(buttons)
	_ = newButton.SetText("Create New")
	cancelButton, _ := walk.NewPushButton(buttons)
	_ = cancelButton.SetText("Cancel")
	_ = dialog.SetDefaultButton(linkButton)
	_ = dialog.SetCancelButton(cancelButton)

	choice := relationChoice{}
	accepted := false
	linkButton.Clicked().Attach(func() {
		index := combo.CurrentIndex()
		if index < 0 || index >= len(model.people) {
			return
		}
		choice.personID = model.people[index].ID
		accepted = true
		dialog.Accept()
	})
	newButton.Clicked().Attach(func() {
		input, created := showPersonDialog(owner, newTitle, nil)
		if !created {
			return
		}
		choice.newPerson = &input
		accepted = true
		dialog.Accept()
	})
	cancelButton.Clicked().Attach(dialog.Cancel)
	dialog.Run()
	return choice, accepted
}

func optionalDate(edit *walk.DateEdit) *time.Time {
	value := edit.Date()
	if value.IsZero() {
		return nil
	}
	value = calendarDate(value)
	return &value
}

func calendarDate(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}
