package main

import (
	"os/exec"
	"os"
	"fmt"
	//"time"
	"github.com/fsouza/go-dockerclient"
	"github.com/codegangsta/cli"
	"errors"
	"io"
)

var dService = &DockerService{}

type DockerService struct {
	client 	*docker.Client
}

func (this *DockerService) init() error {
	// TODO get the information automatically from docker-machine env default
	endpoint := "tcp://192.168.99.101:2376"
    path := "/Users/denislavrov/.docker/machine/machines/default"
    ca := fmt.Sprintf("%s/ca.pem", path)
    cert := fmt.Sprintf("%s/cert.pem", path)
    key := fmt.Sprintf("%s/key.pem", path)
    client, err := docker.NewTLSClient(endpoint, cert, key, ca)
	this.client = client
	return err
}

func forwardX11Socket(){
	// TODO rewrite using native go code, take some code from here https://github.com/matthieudelaro/nut
	display := os.Getenv("DISPLAY")
	if display == "" {
		fmt.Println("Display variable was not set, check your x11")
		os.Exit(-1)
	}
	// MUST disable naggle for local forwarding!
	cmd := exec.Command("socat", "TCP-LISTEN:6000,reuseaddr,fork,nodelay", fmt.Sprintf("UNIX-CLIENT:\"%s\"", display))
	go cmd.Run()
}

func newApp(image string, container_name string){
	client := dService.client
	config := docker.Config{Image:image, Env:[]string{"DISPLAY=192.168.99.1:0"}}
	opts := docker.CreateContainerOptions{Name:container_name, Config:&config}
	container, err := client.CreateContainer(opts)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	err = client.StartContainer(container.ID, nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func runApp(container_name string){
	client := dService.client
	container, err := containerByName(container_name)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	err = client.StartContainer(container.ID, nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func containerByName(container_name string) (*docker.APIContainers, error){
	containers, err := dService.client.ListContainers(docker.ListContainersOptions{All:true})
	if err != nil {
		return nil, err
	}
	container_name = "/" + container_name
	for _, container := range containers{
		for _, name := range container.Names{
			if name == container_name {
				return &container, nil
			}
		}
	}

	return nil, errors.New("Container by that name was not found")
}

func checkChanges(containerID string){
	changes, err := dService.client.ContainerChanges(containerID)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	if len(changes) == 0 {
		fmt.Println(0)
	} else {
		for _, change := range changes{
			fmt.Println(change.Path, change.Kind)
		}
	}
}

/*
 This function is for storing the app configuration once you are happy with it
 This will not happen automatically, it is up to the user to call this, as a result
 a new image will be create that can be used later to distribute it, and keep the configuration
 this is somewhat of a service snapshoting feature.
*/
func exportApp(containerID string, outputStream io.Writer){
	image, err := dService.client.CommitContainer(docker.CommitContainerOptions{Container:containerID})
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	err = dService.client.ExportImage(docker.ExportImageOptions{Name:image.ID, OutputStream:outputStream})
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func main(){
	err := dService.init()
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

	app := cli.NewApp()
  	app.Name = "toolcase"
  	app.Usage = "carry your tools with you"

	changes := cli.Command{
    Name:        "changes",
    Usage:       "supply container id as argument",
    Action: func(c *cli.Context) error {
      	checkChanges(c.Args()[0])
      	return nil
    }}

	new := cli.Command{
    Name:        "new",
    Usage:       "image_name container_name (container_name is up to you)",
    Action: func(c *cli.Context) error {
		if len(c.Args()) == 2 {
			forwardX11Socket()
			newApp(c.Args()[0], c.Args()[1])
			return nil
		}
      	return errors.New("wrong arguments")
    }}

	run := cli.Command{
    Name:        "run",
    Usage:       "supply container_name as agument",
    Action: func(c *cli.Context) error {
		forwardX11Socket()
		runApp(c.Args()[0])
      	return nil
    }}

	export := cli.Command{
    Name:        "export",
    Usage:       "supply container_name file_out as aguments",
    Action: func(c *cli.Context) error {
		container, err := containerByName(c.Args()[0])
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
		f, err := os.Create(c.Args()[1])
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
		exportApp(container.ID, f)
      	return nil
    }}

	app.Commands = append(app.Commands, changes)
	app.Commands = append(app.Commands, new)
	app.Commands = append(app.Commands, run)
	app.Commands = append(app.Commands, export)
	app.Run(os.Args)
}