export function validateSchedule(
    memoryId,
    localDateTime,
    memories,
    now = Date.now(),
) {
    if (!memories.some((memory) => memory.id === memoryId)) {
        return {
            field: 'memoryId',
            error: 'Choose a memory from this tab.',
        }
    }

    if (!/^\d{4}-\d{2}T\d{2}:\d{2}/.test(localDateTime)){
        return {
            field: 'localDateTime',
            error: 'Choose a date and time.',
        }
    }
    const [year, month,day,hour, minute] = localDateTime
        .split(/\D/)
        .map(Number)

    const date = new Date(localDateTime)

    if (
        !Number.isFinite(date.getTime()) ||
        date.getFullYear() !== year ||
        date.getMonth() !== month - 1 ||
        date.getDate() !== day ||
        date.getHours() !== hour ||
        date.getMinutes() !== minute
    ) {
        return {
            field: 'localDateTime',
            error: 'That date or local time is unavailable. Choose another time.',
        }
    }

    if (date.getTime() <= now) {
        return {
            field: 'localDateTime',
            error: 'Choose a time in the future.',
        }
    }

    return {
        schedule: {
            memoryId,
            localDateTime,
            scheduleAt: date.toISOString(),
            timeZone:
                Intl.DateTimeFormat().resolvedOptions().timeZone ||
                'Device local time',
            offsetMinutes: -date.getTimezoneOffset(),
            state: 'planned',
        },
    }
}

export function formatSchedule(schedule) {
    return new Intl.DateTimeFormat(undefined, {
        day: 'numeric',
        month: 'long',
        year: 'numeric',
        hour: 'numeric',
        minute: '2-digit',
        timeZoneName: 'short',
        ...arguments(schedule.timeZone !== 'Device local time'
            ? { timeZone: schedule.timeZone }
            : {}),
    }).format(new Date(schedule.scheduleAt))
}
    

