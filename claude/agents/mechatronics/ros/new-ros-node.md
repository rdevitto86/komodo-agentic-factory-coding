# Skill: /new-ros-node

Scaffold a new ROS 2 node (C++ or Python) following project conventions.

## Usage

```
/new-ros-node <package> <node-name> [--cpp|--python] [--lifecycle] [--timer <hz>]
```

- `<package>` — ROS 2 package name in snake_case, e.g. `sensor_driver`, `motion_controller`.
- `<node-name>` — Node name in snake_case, e.g. `lidar_publisher`, `joint_state_broadcaster`.
- `--cpp` or `--python` — Implementation language (default: `--cpp`).
- `--lifecycle` — Generate a `LifecycleNode` instead of a plain `Node`. Use for hardware drivers and any node that benefits from managed state transitions.
- `--timer <hz>` — Add a periodic timer callback running at `<hz>` Hz (e.g. `--timer 50`).

**Must be run from inside the ROS 2 workspace root.**

---

## Before generating anything

1. List `src/<package>/` — check if the package already exists. If it does, only add the new node files; do not regenerate `package.xml` or `CMakeLists.txt` from scratch.
2. If the package exists, read `src/<package>/CMakeLists.txt` to understand how existing nodes are registered.
3. Read `src/<package>/package.xml` to confirm existing dependencies before adding new ones.

---

## Files to generate (new package)

### `src/<package>/package.xml`

```xml
<?xml version="1.0"?>
<package format="3">
  <name><package></name>
  <version>0.1.0</version>
  <description>TODO: describe the package</description>
  <maintainer email="TODO@example.com">TODO</maintainer>
  <license>TODO</license>

  <buildtool_depend>ament_cmake</buildtool_depend>

  <depend>rclcpp</depend>
  <!-- TODO: add message/service dependencies -->

  <test_depend>ament_lint_auto</test_depend>
  <test_depend>ament_lint_common</test_depend>

  <export>
    <build_type>ament_cmake</build_type>
  </export>
</package>
```

---

### `src/<package>/CMakeLists.txt`

```cmake
cmake_minimum_required(VERSION 3.8)
project(<package>)

find_package(ament_cmake REQUIRED)
find_package(rclcpp REQUIRED)
# TODO: find_package for additional dependencies

add_executable(<node_name> src/<node_name>.cpp)
ament_target_dependencies(<node_name> rclcpp)

install(TARGETS <node_name>
  DESTINATION lib/${PROJECT_NAME})

ament_package()
```

---

### `src/<package>/src/<node_name>.cpp` (C++, plain node)

```cpp
#include <rclcpp/rclcpp.hpp>

class <NodeClass> : public rclcpp::Node {
public:
  <NodeClass>() : Node("<node_name>") {
    RCLCPP_INFO(get_logger(), "<node_name> started");
    // TODO: initialize publishers, subscribers, services, timers
  }

private:
  // TODO: member variables
};

int main(int argc, char * argv[]) {
  rclcpp::init(argc, argv);
  rclcpp::spin(std::make_shared<<NodeClass>>());
  rclcpp::shutdown();
  return 0;
}
```

**Lifecycle variant** — extend `rclcpp_lifecycle::LifecycleNode` and implement `on_configure`, `on_activate`, `on_deactivate`, `on_cleanup`, `on_shutdown` callbacks.

**Timer variant** — add in constructor:
```cpp
timer_ = create_wall_timer(
  std::chrono::milliseconds(1000 / <hz>),
  std::bind(&<NodeClass>::timer_callback, this));
```

---

### `src/<package>/src/<node_name>.py` (Python)

```python
import rclpy
from rclpy.node import Node

class <NodeClass>(Node):
    def __init__(self):
        super().__init__('<node_name>')
        self.get_logger().info('<node_name> started')
        # TODO: initialize publishers, subscribers, services, timers

def main(args=None):
    rclpy.init(args=args)
    node = <NodeClass>()
    rclpy.spin(node)
    node.destroy_node()
    rclpy.shutdown()

if __name__ == '__main__':
    main()
```

---

## After generating

1. Print all files created.
2. Remind the developer to:
   - Run `colcon build --packages-select <package>` to verify the build.
   - Source the workspace: `. install/setup.bash`.
   - Launch the node: `ros2 run <package> <node_name>` to verify it starts cleanly.
   - Add the node to a launch file if it will run as part of a larger system.
   - For lifecycle nodes: test each state transition explicitly before integrating.
