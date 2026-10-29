# Process history

Vigil retains the latest 60 successful CPU/memory observations per active process during the current TUI session. Allow several refreshes, select a process, and press `i` to see session trends in the evidence overview. Tab shows the timestamped samples in JSON. Confirming an interpretation includes this same reviewed history.

CPU uses one logical CPU as 100%, so a multithreaded process may exceed 100%. Memory is a percentage of total system RAM; its change is expressed in percentage points. Each graph uses its own relative scale. Average and peak CPU describe retained samples, not a continuous or time-weighted measurement.

History is stored only in memory. It is discarded when a process disappears from the collected list or its start time changes, and when Vigil exits. The first observation establishes identity without recording a CPU delta. Failed readings are skipped. Refresh intervals vary with collection time; pausing and unavailable metrics create gaps visible in the timestamps. This is observational evidence, not a trusted baseline or proof of a leak or root cause.

CLI `--inspect` is a single investigation and does not have TUI session history. The feature does not add persistent recording or automatic anomaly detection.
