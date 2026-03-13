% Factorial
factorial(0, 1).
factorial(N, F) :-
    N > 0,
    N1 is N - 1,
    factorial(N1, F1),
    F is N * F1.

% Fibonacci
fib(0, 0).
fib(1, 1).
fib(N, F) :-
    N > 1,
    N1 is N - 1,
    N2 is N - 2,
    fib(N1, F1),
    fib(N2, F2),
    F is F1 + F2.

% List sum
sum_list([], 0).
sum_list([H|T], S) :-
    sum_list(T, S1),
    S is S1 + H.

% List max
max_list([X], X).
max_list([X|Xs], Max) :-
    max_list(Xs, MaxRest),
    (X > MaxRest -> Max = X ; Max = MaxRest).
