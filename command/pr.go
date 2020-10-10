package command

import (
  "fmt"
  "github.com/npathai/github-cli-clone/git"
  "github.com/npathai/github-cli-clone/github"
  "github.com/spf13/cobra"
)

func init() {
  RootCmd.AddCommand(prCmd)
  prCmd.AddCommand(prListCmd)
}

var prCmd = &cobra.Command{
  Use: "pr",
  Short: "Work with pull requests",
  Long: "This command allows you to work with pull requests",
  Args: cobra.MinimumNArgs(1),
  Run: func(cmd *cobra.Command, args []string) {
    fmt.Println("pr")
  },
}

var prListCmd = &cobra.Command{
  Use: "list",
  Short: "List pull requests",
  Run: func(cmd *cobra.Command, args []string) {
    ExecutePr()
  },
}

type prFilter int

const (
createdByViewer prFilter = iota
reviewRequested
)

func ExecutePr() {
  prsCreatedByViewer := pullRequests(createdByViewer)

  fmt.Printf("count! %d\n", len(prsCreatedByViewer))
}

func pullRequests(filter prFilter)[]github.PullRequest {
  project := project()
  client := github.NewClient(project.Host)
  currentBranch, err := git.Head()
  if err != nil {
    panic(err)
  }

  headWithOwner := fmt.Sprintf("%s:%s", project.Owner, currentBranch)
  filterParams := map[string]interface{}{"headWithOwner": headWithOwner}
  prs, err := client.FetchPullRequests(&project, filterParams, 10, nil)
  if err != nil {
    panic(err)
  }

  return prs
}

func project() github.Project {
  remotes, error := github.Remotes()
  if error != nil {
    panic(error)
  }

  for _, remote := range remotes {
    if project, error := remote.Project(); error == nil {
      return *project
    }
  }

  panic("Could not get the project. What is a project? I don't know, it's kind of like a git repository I think?")
}