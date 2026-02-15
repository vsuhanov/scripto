I want to work on restructuring how the application works, I am not quite happy with the fact that I have commands and then there are bubble tea models. 

I think that I am misusing some of the aspects of the bubble tea and need to structure those better.

For the root commmand (and well others too) what confuses me all the time is how the domain model is used (in my case it's always Script entity almost never something else). Like I'd like there to be a central service that performs any operations related to it - store/update/delete etc. 

Then I think I want to have a central execution service (which kind of exists (execution.NewScriptExecutor()) but it all over the place. My root_flow_controller and commands/root.go both do some stuff with executor or the command duplicates logic of the executor. 

like root_flow_controller executeFoundScript does ... like who knows what? and if I want to run a particular Script entity I have no idea how to call it in any other place. 

Regarding the executor there is a thing to consider - they way the app is implemented the exucution is not exactly real - it's just write some files that are going to be sourced by the wrapper - and especially because of this complex logic I want to keep it somewhere else


# Updates

## Branch
Feature branch for this work: `refactor/root-flow-controller`


## Architecture Decision

- `scripto` → TUI at MainListScreen
- `scripto add` → TUI at HistoryScreen (handled in root.go, not separate command)
- `scripto install` and `scripto completions` remain separate commands
- **Delete** `commands/add.go` and all command-line flags for adding scripts
- All add functionality goes through TUI now (simpler, more consistent)
- Single RootModel with different start modes

