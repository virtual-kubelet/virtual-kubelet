pub trait Filter<T> {
    fn xfilter<P: FnOnce(&T) -> bool>(self, predicate: P) -> Self;
}

impl<T> Filter<T> for Option<T> {
    fn xfilter<P: FnOnce(&T) -> bool>(self, predicate: P) -> Self {
        match self {
            Some(x) => {
                if predicate(&x) {
                    Some(x)
                } else {
                    None
                }
            }
            None => None,
        }
    }
}
