// task-dag.js
// Minimal in-memory task DAG view from a log stream.

const TASK_STATUS = {
  OPEN: 'open',
  DONE: 'done'
}

function createTaskDag() {
  const tasks = new Map()

  function ensureTask(taskId) {
    if (!tasks.has(taskId)) {
      tasks.set(taskId, {
        id: taskId,
        title: '',
        topic: '',
        dependencies: new Set(),
        tags: new Set(),
        signaturesRequired: 1,
        signatures: new Set(),
        status: TASK_STATUS.OPEN
      })
    }
    return tasks.get(taskId)
  }

  function applyEntry(entry) {
    if (!entry || !entry.type) return

    switch (entry.type) {
      case 'task.add': {
        const task = ensureTask(entry.taskId)
        task.title = entry.title || task.title
        task.topic = entry.topic || task.topic
        task.signaturesRequired = entry.signatures_required || task.signaturesRequired
        break
      }
      case 'task.dep.add': {
        const task = ensureTask(entry.taskId)
        task.dependencies.add(entry.dependsOn)
        break
      }
      case 'task.tag.add': {
        const task = ensureTask(entry.taskId)
        task.tags.add(entry.tag)
        break
      }
      case 'task.sign': {
        const task = ensureTask(entry.taskId)
        if (entry.signer) task.signatures.add(entry.signer)
        break
      }
      default:
        break
    }

    updateStatus()
  }

  function updateStatus() {
    for (const task of tasks.values()) {
      const depsDone = [...task.dependencies].every((depId) => {
        const dep = tasks.get(depId)
        return dep && dep.status === TASK_STATUS.DONE
      })

      const signaturesDone = task.signatures.size >= task.signaturesRequired

      if (depsDone && signaturesDone) {
        task.status = TASK_STATUS.DONE
      }
    }
  }

  function getTask(taskId) {
    return tasks.get(taskId)
  }

  function listTasks() {
    return [...tasks.values()].map((task) => ({
      ...task,
      dependencies: [...task.dependencies],
      tags: [...task.tags],
      signatures: [...task.signatures]
    }))
  }

  return { applyEntry, getTask, listTasks }
}

module.exports = { createTaskDag }
