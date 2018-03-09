[![Build Status](https://travis-ci.org/dedis/cothority_template.svg?branch=master)](https://travis-ci.org/dedis/cothority_template)

# Template for a new cothority protocol/service/app

Building on the ONet-library available at 
https://github.com/dedis/onet, this
repo holds templates to build the different parts necessary for a cothority
addition:

* protocol - define an ephemeral, distributed, decentralized protocol
* service - create a long-term service that can spawn any number of protocols
* app - write an app that will interact with, or spawn, a cothority

This repo is geared towards PhD-students who want to add a new functionality to
the cothority by creating their own protocols, services and apps.

## Starting

You can go-get the repo, then start your project on a new branch. This allows
you to follow the main cothority-template in case something needs to be
updated. We suppose you already forked the cothority-template repo into your
account at `yourlogin`.

```bash
go get -u github.com/dedis/cothority_template
cd $GOPATH/src/github.com/dedis/cothority_template
git remote add perso git@github.com/yourlogin/template
git checkout -b my_new_project
git push -u perso my_new_project
```

Now you can do all your development in `$GOPATH/src/github.com/dedis/cothority_template`
until you are proficient enough to move it either to the main-repository at
`dedis/cothority` or adjusting the paths and publish it under your own repo.

## Documentation

You find more documentation on how to use the template on the wiki:
[Cothority Template](https://github.com/dedis/cothority_template/wiki)

More documentation and examples can be found at:
- To run and use a conode, have a look at 
	[Cothority Node](https://github.com/dedis/cothority/wiki)
	with examples of protocols, services and apps
- To participate as a core-developer, go to 
	[Cothority Network Library](https://github.com/dedis/onet/wiki)

## License

All repositories for the cothority are double-licensed under a 
GNU/AGPL 3.0 and a commercial license. If you want to have more information, 
contact us at dedis@epfl.ch.

## Contribution

If you want to contribute to Cothority-ONet, please have a look at 
[CONTRIBUTION](https://github.com/dedis/cothority/blob/master/CONTRIBUTION) for
licensing details. Once you are OK with those, you can have a look at our
coding-guidelines in
[Coding](https://github.com/dedis/Coding). In short, we use the github-issues
to communicate and pull-requests to do code-review. Travis makes sure that
everything goes smoothly. And we'd like to have good code-coverage.

# Contact

You can contact us at https://groups.google.com/forum/#!forum/cothority