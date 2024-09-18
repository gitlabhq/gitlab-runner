
<!--

Update milestone placeholders below

-->


## :paperclips: Cross Functional Programs

## :runner: Runner Core

#### :bug: Bugs ~"Runner::P1" 

```glql
---
display: table
fields: title, epic, assignees, healthStatus, state
---

project="gitlab-org/gitlab-runner" and milestone = "%%.%" and label = ("type::bug", "Category:Runner Core")

```

#### :sparkles:  Features ~"Runner::P1" 

```glql
---
display: table
fields: title, epic, assignees, healthStatus, state
---

project="gitlab-org/gitlab-runner" and milestone = "%%.%" and label= "type::feature" and label="Category:Runner Core"

```

#### :tools: Maintenance ~"Runner::P1" 

```glql
---
display: table
fields: title, epic, assignees, healthStatus, state
---

project="gitlab-org/gitlab-runner" and milestone = "%%.%" and label= "type::maintenance" and label="Category:Runner Core"

```

~Stretch 

```glql
---
display: table
fields: title, epic, assignees, healthStatus, state
---

project="gitlab-org/gitlab-runner" and milestone = "%%.%" and label= "stretch" and label="Category:Runner Core"

```

## :roller_coaster: Runner Fleet

#### :bug: Bugs ~"Runner::P1" 

```glql
---
display: table
fields: title, epic, assignees, healthStatus, state
---

project="gitlab-org/gitlab" and milestone = "%%.%" and label= "type::bug" and label="Fleet Visibility" 

```

#### :sparkles:  Features ~"Runner::P1" 

```glql
---
display: table
fields: title, epic, assignees, healthStatus, state
---

project="gitlab-org/gitlab" and milestone = "%%.%" and label= "type::feature" and label="Category:Fleet Visibility" 

```

#### :tools: Maintenance ~"Runner::P1" 

```glql
---
display: table
fields: title, epic, assignees, healthStatus, state
---

project="gitlab-org/gitlab" and milestone = "%%.%" and label= "type::maintenance" and label="Category:Fleet Visibility" 

```

~Stretch 

```glql
---
display: table
fields: title, epic, assignees, healthStatus, state
---

project="gitlab-org/gitlab" and milestone = "%%.%" and label= "stretch" and label="Category:Fleet Visibility" 

```