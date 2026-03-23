# Candidate Feedback

## Anand

Anand correctly stated that they are checked at compile time and that working with streams requires explicit handling and wrapping, but only the first statement earned a point.

Anand vaguely mentioned the flexibility of reading the data and that it is used with collections to enable polymorphism.

Anand quickly identified a viable approach to solve the problem. They initially included the obstacle matrix as an input parameter, but the interviewer asked whether it was possible to proceed using the data already available in the `RunCollection`. Anand then worked steadily through the approach and produced a complete solution that ran correctly after fixing a few typos and the method signature. They also described one test with more obstacles and runs, and another covering the minimum and maximum obstacle-time values.

---

## Dheeraj

Dheeraj correctly explained that the first print statement outputs `10` because method lookup for `count` resolves to the definition in `Apple`. He also correctly noted that `__color` would raise an `AttributeError`, though his explanation was incorrect: he said the method was being called with the wrong format. Dheeraj ran out of time before reaching the third print statement.

Dheeraj quickly identified a viable approach to solve the problem. He then worked steadily through it and produced a complete solution that ran correctly after fixing a few syntax issues related to indentation, along with a typo.

---

## Kevin

Kevin quickly identified a viable approach. They implemented the `addWorkout` function early on, but did not perform validation of `memberID`. They tried to test `addWorkout` in isolation, but ran into errors because `getAverageWorkoutDurations` was still unimplemented, so they moved on to implement the second function and planned to test everything at the end. Along the way, they encountered a few syntax errors involving the map, lambdas, and several typos.

When time ran out, the core algorithm was in place, but they still needed to finish the `memberID` validation, correct the lambda used to calculate the average workout duration, and fix the returned value.

---

## Madhav

Madhav identified the bug, but spent a significant amount of time debugging code that was already a valid fix. He tried several changes to the `personalBest()` stream, including replacing `min` with `max` and removing `orElse()`. He also modified `addRun()` in an attempt to address the issue. When time ran out, he was still searching for a viable fix.

Madhav incorrectly said that the `if` statement could be simplified so that the condition was just `!value`, and suggested introducing a local variable instead of returning directly.

Madhav incorrectly stated that this allows you to acknowledge when an exception occurs so it can be treated as an advantage, and that some exceptions might not be handled properly.

---

## Keerthivasan

Keerthivasan correctly identified that map keys must be immutable, but gave a vague explanation. He said that mutable keys would prevent the map from identifying the right key and would not allow iteration over it. The stronger underlying reasons are that keys identify particular values — so mutating them can cause those values to be overwritten or lost — and that maps, as part of the collections framework, rely on keys remaining stable to support reliable iteration.
