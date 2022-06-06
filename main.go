package main

import (
	"context"
	"fmt"
	"os"
	"io"
	"bytes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"k8s.io/client-go/tools/clientcmd"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// probably better to have a single model own this
var (
	clientWidth int = 0
	clientHeight int = 0
)


func getConfig() *rest.Config {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	configOverrides := &clientcmd.ConfigOverrides{}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		panic(err.Error())
	}
	return config
}

func getClientSet() *kubernetes.Clientset {
	config := getConfig()

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	return clientset
}

func getNamespaceList() tea.Msg {
	clientset := getClientSet()

	namespaceList, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	return namespaceListMsg(namespaceList.Items)
}

type namespaceListMsg []corev1.Namespace

type model struct {
	choices   []corev1.Namespace
	cursor    int
	populated bool
}

func (m model) Init() tea.Cmd {
	return getNamespaceList
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case namespaceListMsg:
		m.choices = msg
		m.populated = true
	case tea.WindowSizeMsg:
		clientWidth = msg.Width
		clientHeight = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}

		case "r":
			return model{}, getNamespaceList

		case "l", "enter":
			if len(m.choices) - 1 < m.cursor{
				break
			}
			namespace := m.choices[m.cursor].Name
			return podModel{from: m, namespace: namespace}, func() tea.Msg {
				return getPodList(namespace)
			}
		}
	}

	return m, nil
}

func (m model) View() string {
	s := "Select Namespace:\n\n"

	for i, namespace := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		s += fmt.Sprintf("%s %s\n", cursor, namespace.Name)
	}

	if !m.populated {
		s += "\nRetrieving Namespaces\n"
	}

	return s + "\n\nPress q to quit, r to refresh"
}

func getPodList(namespace string) tea.Msg {
	clientset := getClientSet()

	podList, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	return podListMsg(podList.Items)
}

type podListMsg []corev1.Pod

type podModel struct {
	choices   []corev1.Pod
	cursor    int
	populated bool
	namespace string
	from tea.Model
}

func (m podModel) Init() tea.Cmd {
	return nil
}

func (m podModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case podListMsg:
		m.choices = msg
		m.populated = true
	case tea.WindowSizeMsg:
		clientWidth = msg.Width
		clientHeight = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "r":
			m.cursor = 0
			return m, func() tea.Msg {
				return getPodList(m.namespace)
			}
		case "l", "enter":
			if len(m.choices) - 1 < m.cursor{
				break
			}
			return containerModel{from: m, namespace: m.namespace, pod: m.choices[m.cursor].Name}, func() tea.Msg {
				return getContainerList(m.choices[m.cursor])
			}
		case "h":
			return m.from, nil
		}
	}
	return m, nil
}

func (m podModel) View() string {
	s := "Select Pod:\n\n"

	for i, pod := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		s += fmt.Sprintf("%s %s\n", cursor, pod.Name)
	}

	return s + "\n\nPress q to quit, r to refresh, h to go back"
}

func getContainerList(pod corev1.Pod) tea.Msg {
	var containers []corev1.Container
	var types []string
	for _, container := range pod.Spec.InitContainers {
		containers = append(containers, container)
		types = append(types, "init container")
	}
	for _, container := range pod.Spec.Containers {
		containers = append(containers, container)
		types = append(types, "container")
	}
	return containerListMsg{containers: containers, types: types}
}

type containerModel struct {
	choices   []corev1.Container
	types []string
	cursor    int
	populated bool
	pod string
	namespace string
	from tea.Model
}

type containerListMsg struct {
	containers []corev1.Container
	types []string
}

func (m containerModel) Init() tea.Cmd {
	return nil
}

func (m containerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case containerListMsg:
		m.choices = msg.containers
		m.types = msg.types
		m.populated = true
	case tea.WindowSizeMsg:
		clientWidth = msg.Width
		clientHeight = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "h":
			return m.from, nil
		case "l", "enter":
			if len(m.choices) - 1 < m.cursor{
				break
			}
			return containerLogModel{from: m}, func() tea.Msg {
				return getContainerLogs(m.namespace, m.pod, m.choices[m.cursor].Name)
			}
		}
	}
	return m, nil
}

func (m containerModel) View() string {
	s := "Select Container:\n\n"

	for i, container := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		s += fmt.Sprintf("%s %s\n\t%s\n", cursor, container.Name, lipgloss.NewStyle().Foreground(lipgloss.Color("44")).Render(m.types[i]))
	}

	return s + "\n\nPress q to quit, h to go back"
}

func getContainerLogs(namespace, pod, container string) tea.Msg {
	clientset := getClientSet()
	req := clientset.CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{Container: container})
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return containerLogMsg("Error in opening log stream")
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return containerLogMsg("Error copying logs from stream")
	}
	str := buf.String()
	return containerLogMsg(str)
}

type containerLogModel struct {
	viewport viewport.Model
	ready bool
	from tea.Model
}

type containerLogMsg string

func (m containerLogModel) Init() tea.Cmd {
	return nil
}

func (m containerLogModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type){
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "h":
			return m.from, nil
		}
	case containerLogMsg:
		m.viewport = viewport.New(clientWidth, clientHeight)
		m.ready = true
		m.viewport.SetContent(string(msg))
	}
	if m.ready {
		updated_viewport, cmd := m.viewport.Update(msg)
		m.viewport = updated_viewport
		return m, cmd
	}
	return m, nil
}

func (m containerLogModel) View() string {
	if (m.ready){
		return m.viewport.View()
	}
	return "Retrieving logs ..."
}

func main() {
	p := tea.NewProgram(model{}, tea.WithAltScreen())

	if err := p.Start(); err != nil {
		fmt.Printf("There's an error: %v", err)
		os.Exit(1)
	}
}
