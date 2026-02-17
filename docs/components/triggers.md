# inGitDB Triggers

Triggers are pluggable scripts, executable and webhooks
that can be called when data change.

Some triggers, like web-hooks, are built-in and can be configured within inGitDB collection definition.

Other triggers are just executables receiving events (though `stdin|HTTP|pipe`) and can do anything you want. 