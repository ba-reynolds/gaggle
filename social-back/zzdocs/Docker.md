## Definitions

### DockerFile

A Dockerfile is a text-based script containing a set of instructions used by the docker build command to create a **Docker image**. Each instruction in the Dockerfile adds a layer to the image, which collectively define the file system, environment variables, default commands, and other configurations necessary to instantiate a container. It serves as a deterministic and repeatable specification for the image's content and behavior.

```dockerfile
# Use an official Python runtime as the base image
# This will also implicitly include an operating system layer
FROM python:3.9-slim

# Set the working directory inside the container
# It's akin to using `cd /app` inside a shell but also ensures
# that if `/app` doesn’t exist, it will be created automatically.
WORKDIR /app

# Copies the entire contents of the current directory (where the
# Dockerfile resides on the host machine) into the container's `/app` directory.
COPY . /app

# Install dependencies listed in requirements.txt
RUN pip install --no-cache-dir -r requirements.txt

# Expose port 5000 for the app
EXPOSE 5000

# Define the command to run the application
CMD ["python", "app.py"]
```

### Image

A Docker image is a read-only template created by executing a Dockerfile. It contains the filesystem snapshot, application code, runtime, libraries, and everything needed to run a container. Images are versioned (v1, v2, etc.) and can be shared via Docker registries. You can think of an image as a packaged application environment.

For example, if you build the above Dockerfile with:
```
docker build -t my-python-app .
```

This command creates an image called `my-python-app`. That image can then be instantiated into containers.

### Containers

A Docker container is a runnable instance of a Docker image. Containers are isolated environments that run the image code with their own filesystem, network interface, and process space. Containers can be started, stopped, paused, and deleted without affecting the underlying image.

The container ID is a unique SHA256-derived hash automatically assigned by Docker at creation.

For example, to run a container from the my-python-app image:
```sh
docker run -d -p 5000:5000 --name myapp-container my-python-app
```

Breaking it down:
- `docker run` tells Docker to create and start a new container from a specified image.
- The `-d` flag runs the container in "detached" mode, meaning it runs in the background rather than tying up your terminal.
- `-p 5000:5000` sets up port forwarding between the host and the container. The first 5000 is the port on your local machine (host), and the second 5000 is the port inside the container. This means that any network traffic sent to port 5000 on your host will be forwarded to port 5000 inside the container, allowing you to access the app running inside the container via localhost:5000 or your machine’s IP address.
- `--name myapp-container` assigns a custom name to the running container, which makes it easier to reference later (for example, stopping or inspecting the container by name instead of using a container ID).
- `my-python-app` is the name (or tag) of the Docker image from which the container will be created and run. This image must be present locally or Docker will attempt to pull it from a registry.

## Handy commands

### `build`

This command builds an image from a Dockerfile located in the current directory and tags it with a name and version.
```
docker build -t myapp:1.0 .
```

### `run`

Runs a container from the ubuntu image with an interactive terminal.
```sh
docker run -it ubuntu /bin/bash
```

Arguments:
- `-i`: Keeps STDIN open even if not attached. Used for interactive sessions.
- `-t`: Allocates a pseudo-TTY, giving an interactive shell interface.
- `ubuntu`: the name of the image to use
- `/bin/bash`: the command to execute inside the container

Alternatively you can use the `-d` flag to run the container in the background.
```sh
docker run -d ubuntu /bin/bash
```

### List all containers

```sh
docker ps
```

### Stop a container

```sh
docker stop webserver
```

Where `webserver` is the container's name or ID.

### Remove a container or image

To remove a **stopped** container:
```sh
docker rm webserver
```

To remove an image:
```sh
docker rmi myapp:1.0
```

### Copy files between host and container

To run `docker cp` the container does **not** need to be running, since we are just accessing the container's file system. The argument with a colon in it is assumed to be the container, while the other is a file from the host.

Copy from host to container:
```sh
docker cp ./config.json webserver:/app/config.json
```

Alternatively you can reverse the direction:
```sh
docker cp webserver:/app/config.json ./config.json 
```

### Check resource usage (CPU, memory) of containers}

```sh
docker stats
```

## Container vs process

At the OS level, a process is an instance of a running program. It has its own memory space, system resources (like file descriptors), and runs in user space, while being managed by the kernel. Processes can be single-threaded or multi-threaded, and are isolated from one another via mechanisms like memory protection and user permissions.

A container, on the other hand, is a higher-level abstraction. It is a group of one or more processes that are namespaced and isolated from the rest of the system. Containers run in user space on the host OS kernel and use Linux namespaces and cgroups to achieve process isolation and resource limiting. A container itself is not a process, but it encapsulates one or more processes that behave as if they are running on a separate system.

A container does not run its own operating system kernel. It shares the host OS kernel, but isolates processes using:
- Namespaces (e.g., PID, NET, UTS, IPC, MNT): These make processes inside the container believe they have their own PID space, network interfaces, hostname, inter-process communication, and mount points.
- Control Groups (cgroups): These restrict and account for resource usage (CPU, memory, disk I/O) of containerized processes.

### Container with different OS from the host

A Linux distribution is composed of two main things:
- **The Linux kernel** – This is the core of the OS, responsible for managing hardware, processes, memory, filesystems, etc. All modern distributions use the same upstream Linux kernel, perhaps with some patches or config options.
- **The userland** – This includes everything outside the kernel: shell utilities, system libraries (like `libc`), configuration files in `/etc`, init systems (like `systemd`, `openrc`), package managers (`apt`, `pacman`), and everything in `/usr`, `/bin`, `/lib`, etc.

Suppose you run:
```
docker run -it archlinux:latest bash
```

Here's what happens under the hood:
1. **Container image filesystem**: The Arch Linux Docker image is just a layered tarball snapshot of **the Arch filesystem tree**. It includes directories like `/bin`, `/etc`, `/usr`, and the relevant Arch binaries, libraries, and configuration. It’s built using Arch's tooling, typically via `pacstrap` or equivalent in the image build process.
2. **Host kernel is still Ubuntu's**: Even though you're "inside Arch," any syscalls made by processes in the container (e.g., `read()`, `open()`, `clone()`, etc.) are handled by the host's Ubuntu kernel. If Arch inside the container uses `pacman`, it's still relying on Ubuntu's kernel to do everything underneath: process scheduling, memory allocation, syscalls.
3. **Isolation**: The container’s namespaces make it look like a separate system. For example:
   - It sees its own PID namespace (PID 1 inside the container).
   - It has its own hostname.
   - It mounts its own `/proc`, `/sys`, `/dev`, all scoped to the container view.