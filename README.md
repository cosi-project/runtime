# os-runtime

OS Runtime contains core resource (state) and controller (operator) engine to build operating systems.

## Design

### Resources

A **resource** is a _metadata_ plus opaque _spec_.
Metadata structure is strictly defined, while a spec is transparent to the `os-runtime`.
Metadata defines an address of the resource: (namespace, type, id, version) and additional fields (finalizers, owner, etc.)

### Controllers

A **controller** is a task that runs as a single thread of execution.
A controller has defined _input_ and _outputs_.
Outputs are static and should be defined at the registration time, while inputs are dynamic and might change during controller execution.

A controller is supposed to implement a reconcile loop: for each reconcile event (coming from the runtime) the controller wakes up, checks the inputs,
performs any actions and modifies the outputs.

Controller inputs are resources which controller can read (it can't read resources that are not declared as inputs), and inputs are the resources controller
gets notified about changes:

* `strong` inputs are the inputs controller depends on in a strong way: it has to be notified when inputs are going to be destroyed via finalizer mechanism;
* `weak` inputs are the inputs controller watches, but it doesn't have to do any cleanup when weak inputs are being destroyed.

A controller can modify finalizers of strong controller inputs; any other modifications to the inputs are not permitted.

Controller outputs are resources which controller can write (create, destroy, update):

* `exclusive` outputs are managed by only a single controller; no other controller can modify exclusive resources
* `shared` outputs are resources that are created by multiple controllers, but each specific resource can only be modified by a controller which created that resource

Runtime verifies that only one controller has `exclusive` access to the resource.

### Principles

* simple and structured: impose structure to make things simple.
* avoid conflicts by design: resources don't have multiple entities which can modify them.
* use controller structure as documentation: graph of dependencies between controllers and resources documents system design and current state.
