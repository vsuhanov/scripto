## Storage 
1. store command history in sqlite database, single table with following columns: **id, execution_timestamp, script_id, executed_script, original_script, placeholder_values, working_directory, script_object_definition, executed_script_hash, original_script_hash** 
	1. executed_script and original_script are the actual values while script_object_definition is the value of the entities.Script serialized. placeholder values is also a serialized json that was resolved by the placeholder form.
	2. hashes are necessary for potential deduplication/search of scripts, like if I executed the same version, then have changed the script, the script_id stays the same but it's version is now different but if there are placeholders in it I also want to see which executions were the same. 
	3. each execution should have an id, just for posterity, and an index on the script_id
2. each time a command is executed store what was the command and reference to the file
	1. if the the script changes historic references are still persisted and shown as is
	2. history command should store both the original command with placeholders, resolved command and all the values for the placeholders — see the structure above
3. For all of this to work, scripts need to have some ids, those need to be uuids or something similarly unique. Would be good to have a one time command that migrates them and then the id becomes mandatory parameter of the script entity. 
4. location of the sqlite db should be configurable via the env variable `SCRIPTO_SQLITE_DB_PATH` but default to `~/.scripto/scripto.sqlite`
5. need to devise migration logic — somehow all the migrations need to be applied and stored, what are possible options for that in go.


# Viewing the history
1. command history should be used in following ways: 
	1. in the main list it should be possible to change the order of the list by the last executed by hitting `o` , need to take into account that there will be multiple order modes for all the scripts, I want to later add option to use `o` to iterate over sorting modes: **last execution, frequency, recency, date added(currently there is no data on that), alphabetic** and then to use `O` to iterate over the same but in the ASC mode
	2. in the preview on the main list it should show the last execution time and number of invocations
2. there should be an option to view the history of a single command by hitting `gh`, `gH` — should go to history screen without a filter. 
	1. Need to make sure to separate two terms: 1. History of the shell commands that is passed to the application via `scripto add`, history of command execution — need to come up with proper names for both
	2. this should move to the screen which will be a shared screen for history, it should be able to show history of all the commands, but have filter on top, when viewing history of a single command it should just prepopulate the filter on top
3. option to view full history of all the commands, without any filtering
4. on the history screen there shall be a table where I can iterate over the executions and see the panel with details of that execution — what was the command that executed. The panel will be somewhat expanded in the future to show all kinds of information
5. on the history screen I should be able to hit enter to execute the same command again (executed_script, with all the same placeholder values) and shift+enter to execute the same command but with new placeholder values — this will require some consideration because the service that records the command will need to record this execution too but I need to think how it should do it, there is a chance that frequency of the command for sorting purposes should use the original_script_hash for counts because the command executed from history can be of older version and should not impact the frequency, but also the resulting script may not reflect what is currently in the configuration. 


